package mongorepository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
	mongoerrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors/mongodb"
)

var _ domain.UserRepository = (*Users)(nil)

var _ domain.UserAuditRepository = (*Audits)(nil)

type Users struct {
	users  *mongo.Collection
	cipher domain.SensitiveDataCipher
	lookup domain.EmailLookupHasher
}

type Audits struct {
	collection *mongo.Collection
	cipher     domain.SensitiveDataCipher
}

func New(db *mongo.Database, cipher domain.SensitiveDataCipher, lookup domain.EmailLookupHasher) *Users {
	return &Users{
		users:  db.Collection("users"),
		cipher: cipher,
		lookup: lookup,
	}
}

func (r *Users) Create(ctx context.Context, user domain.User) (domain.User, error) {
	doc, err := r.document(user)
	if err != nil {
		return domain.User{}, apperrors.FromDependency("encrypt user", err)
	}
	if _, err = r.users.InsertOne(ctx, doc); err != nil {
		return domain.User{}, mongoerrors.From(err)
	}
	return user, nil
}

func (r *Users) Get(ctx context.Context, id string) (domain.User, error) {
	return r.find(ctx, bson.M{"_id": id})
}

func (r *Users) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.find(ctx, bson.M{"email_lookup": r.lookup.Lookup([]byte(email))})
}

func (r *Users) List(ctx context.Context, limit int, after string) (domain.Page, error) {
	filter := bson.M{}
	if after != "" {
		filter["_id"] = bson.M{"$gt": after}
	}
	cursor, err := r.users.Find(ctx, filter, options.Find().SetSort(bson.D{{
		Key:   "_id",
		Value: 1,
	}}).SetLimit(int64(limit+1)))
	if err != nil {
		return domain.Page{}, mongoerrors.From(err)
	}
	defer cursor.Close(ctx)
	documents := []userDocument{}
	if err := cursor.All(ctx, &documents); err != nil {
		return domain.Page{}, mongoerrors.From(err)
	}
	users := make([]domain.User, 0, len(documents))
	for _, doc := range documents {
		user, err := r.user(doc)
		if err != nil {
			return domain.Page{}, apperrors.FromDependency("decrypt users", err)
		}
		users = append(users, user)
	}
	page := domain.Page{
		Users: users,
	}
	if len(users) > limit {
		page.Users = users[:limit]
		page.NextCursor = page.Users[limit-1].ID
	}
	return page, nil
}

func (r *Users) Update(ctx context.Context, id string, patch domain.Patch, updatedAt time.Time) (domain.User, error) {
	set := bson.M{"updated_at": updatedAt}
	if patch.Name != nil {
		value, err := r.cipher.Encrypt([]byte(*patch.Name), aad("users", id, "name"))
		if err != nil {
			return domain.User{}, apperrors.FromDependency("encrypt name", err)
		}
		set["name_encrypted"] = value
	}
	if patch.Email != nil {
		value, err := r.cipher.Encrypt([]byte(*patch.Email), aad("users", id, "email"))
		if err != nil {
			return domain.User{}, apperrors.FromDependency("encrypt email", err)
		}
		set["email_encrypted"] = value
		set["email_lookup"] = r.lookup.Lookup([]byte(*patch.Email))
	}
	var doc userDocument
	if err := r.users.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": set}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc); err != nil {
		return domain.User{}, mongoerrors.From(err)
	}
	user, err := r.user(doc)
	if err != nil {
		return domain.User{}, apperrors.FromDependency("decrypt updated user", err)
	}
	return user, nil
}

func (r *Users) Delete(ctx context.Context, id string) error {
	_, err := r.users.DeleteOne(ctx, bson.M{"_id": id})
	return mongoerrors.From(err)
}

func (r *Users) Count(ctx context.Context) (int64, error) {
	count, err := r.users.CountDocuments(ctx, bson.M{}, options.Count().SetHint("_id_"))
	return count, mongoerrors.From(err)
}

func NewAudits(db *mongo.Database, cipher domain.SensitiveDataCipher) *Audits {
	return &Audits{
		collection: db.Collection("user_audits"),
		cipher:     cipher,
	}
}

func (r *Audits) Append(ctx context.Context, event domain.UserAudit) error {
	doc, err := r.document(event)
	if err != nil {
		return apperrors.FromDependency("encrypt audit", err)
	}
	_, err = r.collection.InsertOne(ctx, doc)
	return mongoerrors.From(err)
}

func (r *Users) find(ctx context.Context, filter bson.M) (domain.User, error) {
	var doc userDocument
	if err := r.users.FindOne(ctx, filter).Decode(&doc); err != nil {
		return domain.User{}, mongoerrors.From(err)
	}
	user, err := r.user(doc)
	if err != nil {
		return domain.User{}, apperrors.FromDependency("decrypt user", err)
	}
	return user, nil
}
