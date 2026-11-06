//go:build integration

package mongorepository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	services "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/services"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/cryptography"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/hash"
)

func TestTransactionalEncryptedStorage(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"create update no-op delete"},
		{"audit failure rolls back user"},
		{"concurrent audit chain"},
		{"lookup tampering rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, userCipher, auditCipher, lookup := storageFixture(t)
			repo := New(db, userCipher, lookup)
			audits := NewAudits(db, auditCipher)
			passwords, err := hash.New(hash.Config{})
			require.NoError(t, err)
			service := services.New(repo, passwords, audits, NewTransactions(db.Client()))
			ctx = domain.WithAuditContext(ctx, domain.AuditContext{
				CallerService: "orch",
				RequestID:     "integration-request",
				TraceID:       "0123456789abcdef0123456789abcdef",
			})
			user, err := service.Create(ctx, domain.Create{
				Name:     "Audit Alice",
				Email:    "audit@example.com",
				Password: "correct-horse-battery",
			})
			require.NoError(t, err)
			require.Equal(t, user.CreatedAt, user.UpdatedAt)
			var raw bson.M
			require.NoError(t, db.Collection("users").FindOne(ctx, bson.M{"_id": user.ID}).Decode(&raw))
			require.NotContains(t, raw, "name")
			require.NotContains(t, raw, "email")
			require.Equal(t, lookup.Lookup([]byte(user.Email)), raw["email_lookup"])
			require.NotContains(t, fmt.Sprint(raw), "Audit Alice")
			require.NotContains(t, fmt.Sprint(raw), "audit@example.com")
			actor := domain.Principal{
				UserID: user.ID,
			}
			switch tt.name {
			case "create update no-op delete":
				name := "Changed Alice"
				updated, err := service.Update(ctx, actor, user.ID, domain.Patch{
					Name: &name,
				})
				require.NoError(t, err)
				require.True(t, updated.UpdatedAt.After(user.UpdatedAt))
				same, err := service.Update(ctx, actor, user.ID, domain.Patch{
					Name: &name,
				})
				require.NoError(t, err)
				require.Equal(t, updated.UpdatedAt, same.UpdatedAt)
				require.NoError(t, service.DeleteAuthorized(ctx, actor, user.ID))
				require.NoError(t, service.DeleteAuthorized(ctx, actor, user.ID))
				count, err := db.Collection("user_audits").CountDocuments(ctx, bson.M{"user_id": user.ID})
				require.NoError(t, err)
				require.EqualValues(t, 3, count)
				var event auditDocument
				require.NoError(t, db.Collection("user_audits").FindOne(ctx, bson.M{"user_id": user.ID, "action": "UPDATE"}).Decode(&event))
				plain, err := auditCipher.Decrypt(event.ChangesEncrypted, aad("user_audits", event.ID, "changes"))
				require.NoError(t, err)
				var changes map[string]struct {
					Before *string `json:"before"`
					After  *string `json:"after"`
				}
				require.NoError(t, json.Unmarshal(plain, &changes))
				require.Equal(t, "Audit Alice", *changes["name"].Before)
				require.Equal(t, "Changed Alice", *changes["name"].After)
				require.Len(t, changes, 1)
				require.Equal(t, user.ID, event.ActorID)
				require.Equal(t, "integration-request", event.RequestID)
			case "audit failure rolls back user":
				require.NoError(t, db.RunCommand(ctx, bson.D{{
					Key:   "collMod",
					Value: "user_audits",
				}, {
					Key:   "validator",
					Value: bson.M{"$expr": bson.M{"$eq": bson.A{1, 0}}},
				}}).Err())
				name := "Must Not Persist"
				_, err := service.Update(ctx, actor, user.ID, domain.Patch{
					Name: &name,
				})
				require.ErrorIs(t, err, apperrors.ErrUnavailable)
				after, err := repo.Get(ctx, user.ID)
				require.NoError(t, err)
				require.Equal(t, user.Name, after.Name)
				require.Equal(t, user.UpdatedAt, after.UpdatedAt)
				count, err := db.Collection("user_audits").CountDocuments(ctx, bson.M{"user_id": user.ID})
				require.NoError(t, err)
				require.EqualValues(t, 1, count)
			case "lookup tampering rejected":
				_, err := db.Collection("users").UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": bson.M{"email_lookup": fmt.Sprintf("%064d", 0)}})
				require.NoError(t, err)
				_, err = repo.Get(ctx, user.ID)
				require.ErrorIs(t, err, apperrors.ErrUnavailable)
			case "concurrent audit chain":
				var wg sync.WaitGroup
				results := make(chan error, 4)
				for index := range 4 {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						name := fmt.Sprintf("Concurrent %d", i)
						_, err := service.Update(ctx, actor, user.ID, domain.Patch{
							Name: &name,
						})
						results <- err
					}(index)
				}
				wg.Wait()
				close(results)
				for err := range results {
					require.NoError(t, err)
				}
				cursor, err := db.Collection("user_audits").Find(ctx, bson.M{"user_id": user.ID, "action": "UPDATE"}, options.Find().SetSort(bson.D{{
					Key:   "occurred_at",
					Value: 1,
				}}))
				require.NoError(t, err)
				defer cursor.Close(ctx)
				events := []auditDocument{}
				require.NoError(t, cursor.All(ctx, &events))
				require.Len(t, events, 4)
				previous := "Audit Alice"
				for _, event := range events {
					plain, err := auditCipher.Decrypt(event.ChangesEncrypted, aad("user_audits", event.ID, "changes"))
					require.NoError(t, err)
					var changes map[string]struct {
						Before *string `json:"before"`
						After  *string `json:"after"`
					}
					require.NoError(t, json.Unmarshal(plain, &changes))
					require.Equal(t, previous, *changes["name"].Before)
					previous = *changes["name"].After
				}
				after, err := repo.Get(ctx, user.ID)
				require.NoError(t, err)
				require.Equal(t, previous, after.Name)
			}
		})
	}
}

func storageFixture(t *testing.T) (context.Context, *mongo.Database, *cryptography.Cipher, *cryptography.Cipher, *cryptography.LookupHasher) {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is required")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetAuth(options.Credential{
		Username:   "root",
		Password:   os.Getenv("MONGO_TEST_PASSWORD"),
		AuthSource: "admin",
	}).SetTimeout(5 * time.Second))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	db := client.Database(fmt.Sprintf("audit_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		require.NoError(t, db.Drop(cleanup))
		_ = client.Disconnect(cleanup)
		cancel()
	})
	binary := os.Getenv("ENSURE_INDEX_BIN")
	if binary == "" {
		t.Skip("ENSURE_INDEX_BIN is required for database setup")
	}
	command := exec.CommandContext(ctx, binary)
	command.Env = append(os.Environ(),
		"CONFIG_FILE="+os.Getenv("ENSURE_INDEX_CONFIG"),
		"MONGO_URI="+uri,
		"MONGO_DATABASE="+db.Name(),
		"MONGO_USERNAME=root",
		"MONGO_PASSWORD="+os.Getenv("MONGO_TEST_PASSWORD"),
		"MONGO_AUTH_SOURCE=admin",
	)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	userCipher, err := cryptography.New(cryptography.Config{
		ActiveKeyID: "v1",
		Keys:        map[string][]byte{"v1": bytes.Repeat([]byte{1}, 32)},
	})
	require.NoError(t, err)
	auditCipher, err := cryptography.New(cryptography.Config{
		ActiveKeyID: "v1",
		Keys:        map[string][]byte{"v1": bytes.Repeat([]byte{2}, 32)},
	})
	require.NoError(t, err)
	lookup, err := cryptography.NewLookup(cryptography.LookupConfig{
		Key: bytes.Repeat([]byte{3}, 32),
	})
	require.NoError(t, err)
	return ctx, db, userCipher, auditCipher, lookup
}
