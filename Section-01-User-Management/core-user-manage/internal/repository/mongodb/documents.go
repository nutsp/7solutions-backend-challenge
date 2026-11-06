package mongorepository

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
)

type userDocument struct {
	ID             string    `bson:"_id"`
	NameEncrypted  []byte    `bson:"name_encrypted"`
	EmailEncrypted []byte    `bson:"email_encrypted"`
	EmailLookup    string    `bson:"email_lookup"`
	PasswordHash   string    `bson:"password_hash"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

type fieldChangeDocument struct {
	Before *string `json:"before"`
	After  *string `json:"after"`
}

type auditDocument struct {
	ID               string    `bson:"_id"`
	UserID           string    `bson:"user_id"`
	Action           string    `bson:"action"`
	OccurredAt       time.Time `bson:"occurred_at"`
	ActorType        string    `bson:"actor_type"`
	ActorID          string    `bson:"actor_id"`
	CallerService    string    `bson:"caller_service"`
	RequestID        string    `bson:"request_id"`
	TraceID          string    `bson:"trace_id"`
	ChangedFields    []string  `bson:"changed_fields"`
	ChangesEncrypted []byte    `bson:"changes_encrypted"`
}

// JSON encoding an array makes associated-data components unambiguous.
func aad(collection, id, field string) []byte {
	raw, _ := json.Marshal([]string{"user-storage-v2", collection, id, field})
	return raw
}

func (r *Users) document(user domain.User) (userDocument, error) {
	name, err := r.cipher.Encrypt([]byte(user.Name), aad("users", user.ID, "name"))
	if err != nil {
		return userDocument{}, err
	}
	email, err := r.cipher.Encrypt([]byte(user.Email), aad("users", user.ID, "email"))
	if err != nil {
		return userDocument{}, err
	}
	return userDocument{
		ID:             user.ID,
		NameEncrypted:  name,
		EmailEncrypted: email,
		EmailLookup:    r.lookup.Lookup([]byte(user.Email)),
		PasswordHash:   user.PasswordHash,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}, nil
}

func (r *Users) user(doc userDocument) (domain.User, error) {
	name, err := r.cipher.Decrypt(doc.NameEncrypted, aad("users", doc.ID, "name"))
	if err != nil {
		return domain.User{}, err
	}
	email, err := r.cipher.Decrypt(doc.EmailEncrypted, aad("users", doc.ID, "email"))
	if err != nil {
		return domain.User{}, err
	}
	if r.lookup.Lookup(email) != doc.EmailLookup {
		return domain.User{}, errors.New("user data integrity check failed")
	}
	return domain.User{
		ID:           doc.ID,
		Name:         string(name),
		Email:        string(email),
		PasswordHash: doc.PasswordHash,
		CreatedAt:    doc.CreatedAt,
		UpdatedAt:    doc.UpdatedAt,
	}, nil
}

func (r *Audits) document(event domain.UserAudit) (auditDocument, error) {
	changes := make(map[string]fieldChangeDocument, len(event.Changes))
	fields := make([]string, 0, len(event.Changes))
	for field, change := range event.Changes {
		changes[field] = fieldChangeDocument{
			Before: change.Before,
			After:  change.After,
		}
		fields = append(fields, field)
	}
	sort.Strings(fields)
	raw, err := json.Marshal(changes)
	if err != nil {
		return auditDocument{}, err
	}
	sealed, err := r.cipher.Encrypt(raw, aad("user_audits", event.ID, "changes"))
	if err != nil {
		return auditDocument{}, err
	}
	return auditDocument{
		ID:               event.ID,
		UserID:           event.UserID,
		Action:           event.Action,
		OccurredAt:       event.OccurredAt,
		ActorType:        event.Actor.ActorType,
		ActorID:          event.Actor.ActorID,
		CallerService:    event.Actor.CallerService,
		RequestID:        event.Actor.RequestID,
		TraceID:          event.Actor.TraceID,
		ChangedFields:    fields,
		ChangesEncrypted: sealed,
	}, nil
}
