// Package mongoerrors converts MongoDB failures into semantic application errors.
package mongoerrors

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"

	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
)

func From(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return apperrors.ErrNotFound
	}
	if mongo.IsDuplicateKeyError(err) {
		return apperrors.ErrConflict
	}
	return apperrors.FromDependency("mongodb", err)
}
