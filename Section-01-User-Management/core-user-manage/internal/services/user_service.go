package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
)

var _ domain.UserService = (*Users)(nil)

type Users struct {
	repo         domain.UserRepository
	security     domain.PasswordHasher
	audits       domain.UserAuditRepository
	transactions domain.TransactionManager
}

func New(repo domain.UserRepository, security domain.PasswordHasher, audits domain.UserAuditRepository, transactions domain.TransactionManager) *Users {
	return &Users{
		repo:         repo,
		security:     security,
		audits:       audits,
		transactions: transactions,
	}
}

func (s *Users) Create(ctx context.Context, in domain.Create) (domain.User, error) {
	name, ok := domain.NormalizeName(in.Name)
	email, validEmail := domain.NormalizeEmail(in.Email)
	if !ok || !validEmail || !domain.ValidPassword(in.Password) {
		return domain.User{}, apperrors.ErrInvalid
	}
	hash, err := s.security.Hash(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	id, err := newID()
	if err != nil {
		return domain.User{}, err
	}
	auditID, err := newID()
	if err != nil {
		return domain.User{}, err
	}
	var created domain.User
	err = s.transactions.WithinTransaction(ctx, func(tx context.Context) error {
		now := time.Now().UTC().Truncate(time.Millisecond)
		var err error
		created, err = s.repo.Create(tx, domain.User{
			ID:           id,
			Name:         name,
			Email:        email,
			PasswordHash: hash,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if err != nil {
			return err
		}
		actor := domain.AuditActor(tx)
		actor.ActorType = "anonymous"
		actor.ActorID = ""
		return s.audits.Append(tx, domain.UserAudit{
			ID:         auditID,
			UserID:     id,
			Action:     "CREATE",
			OccurredAt: now,
			Actor:      actor,
			Changes: map[string]domain.FieldChange{"name": {
				After: text(name),
			}, "email": {
				After: text(email),
			}},
		})
	})
	if err != nil {
		return domain.User{}, err
	}
	return created, nil
}

func (s *Users) Get(ctx context.Context, p domain.Principal, id string) (domain.User, error) {
	if p.UserID == "" {
		return domain.User{}, apperrors.ErrForbidden
	}
	return s.Internal(ctx, id)
}

func (s *Users) Internal(ctx context.Context, id string) (domain.User, error) {
	if !domain.ValidUserID(id) {
		return domain.User{}, apperrors.ErrInvalid
	}
	return s.repo.Get(ctx, id)
}

func (s *Users) FindByEmail(ctx context.Context, raw string) (domain.User, error) {
	email, ok := domain.NormalizeEmail(raw)
	if !ok {
		return domain.User{}, apperrors.ErrInvalid
	}
	return s.repo.FindByEmail(ctx, email)
}

func (s *Users) List(ctx context.Context, p domain.Principal, limit int, after string) (domain.Page, error) {
	if p.UserID == "" {
		return domain.Page{}, apperrors.ErrForbidden
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 || (after != "" && !domain.ValidUserID(after)) {
		return domain.Page{}, apperrors.ErrInvalid
	}
	return s.repo.List(ctx, limit, after)
}

func (s *Users) Update(ctx context.Context, p domain.Principal, id string, patch domain.Patch) (domain.User, error) {
	if p.UserID == "" || p.UserID != id {
		return domain.User{}, apperrors.ErrForbidden
	}
	if !domain.ValidUserID(id) || (patch.Name == nil && patch.Email == nil) {
		return domain.User{}, apperrors.ErrInvalid
	}
	if patch.Name != nil {
		value, ok := domain.NormalizeName(*patch.Name)
		if !ok {
			return domain.User{}, apperrors.ErrInvalid
		}
		patch.Name = &value
	}
	if patch.Email != nil {
		value, ok := domain.NormalizeEmail(*patch.Email)
		if !ok {
			return domain.User{}, apperrors.ErrInvalid
		}
		patch.Email = &value
	}
	auditID, err := newID()
	if err != nil {
		return domain.User{}, err
	}
	var updated domain.User
	err = s.transactions.WithinTransaction(ctx, func(tx context.Context) error {
		before, err := s.repo.Get(tx, id)
		if err != nil {
			return err
		}
		changes := map[string]domain.FieldChange{}
		actual := domain.Patch{}
		if patch.Name != nil && *patch.Name != before.Name {
			changes["name"] = domain.FieldChange{
				Before: text(before.Name),
				After:  text(*patch.Name),
			}
			actual.Name = patch.Name
		}
		if patch.Email != nil && *patch.Email != before.Email {
			changes["email"] = domain.FieldChange{
				Before: text(before.Email),
				After:  text(*patch.Email),
			}
			actual.Email = patch.Email
		}
		if len(changes) == 0 {
			updated = before
			return nil
		}
		now := time.Now().UTC().Truncate(time.Millisecond)
		if !now.After(before.UpdatedAt) {
			now = before.UpdatedAt.Add(time.Millisecond)
		}
		updated, err = s.repo.Update(tx, id, actual, now)
		if err != nil {
			return err
		}
		actor := domain.AuditActor(tx)
		actor.ActorType = "user"
		actor.ActorID = p.UserID
		return s.audits.Append(tx, domain.UserAudit{
			ID:         auditID,
			UserID:     id,
			Action:     "UPDATE",
			OccurredAt: now,
			Actor:      actor,
			Changes:    changes,
		})
	})
	if err != nil {
		return domain.User{}, err
	}
	return updated, nil
}

func (s *Users) DeleteAuthorized(ctx context.Context, p domain.Principal, id string) error {
	if p.UserID == "" || p.UserID != id {
		return apperrors.ErrForbidden
	}
	if !domain.ValidUserID(id) {
		return apperrors.ErrInvalid
	}
	auditID, err := newID()
	if err != nil {
		return err
	}
	return s.transactions.WithinTransaction(ctx, func(tx context.Context) error {
		before, err := s.repo.Get(tx, id)
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.repo.Delete(tx, id); err != nil {
			return err
		}
		actor := domain.AuditActor(tx)
		actor.ActorType = "user"
		actor.ActorID = p.UserID
		return s.audits.Append(tx, domain.UserAudit{
			ID:         auditID,
			UserID:     id,
			Action:     "DELETE",
			OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
			Actor:      actor,
			Changes: map[string]domain.FieldChange{"name": {
				Before: text(before.Name),
			}, "email": {
				Before: text(before.Email),
			}},
		})
	})
}

func (s *Users) Count(ctx context.Context) (int64, error) {

	return s.repo.Count(ctx)
}

func newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func text(value string) *string {

	return &value
}
