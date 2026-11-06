package mongoerrors

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
)

func TestMongoMappings(t *testing.T) {
	unavailable := errors.New("connection lost")
	for _, tc := range []struct {
		input error
		want  error
	}{
		{nil, nil}, {mongo.ErrNoDocuments, apperrors.ErrNotFound},
		{mongo.WriteException{
			WriteErrors: []mongo.WriteError{{
				Code: 11000,
			}},
		}, apperrors.ErrConflict},
		{context.DeadlineExceeded, apperrors.ErrTimeout}, {context.Canceled, context.Canceled}, {unavailable, apperrors.ErrUnavailable},
	} {
		t.Run(fmt.Sprint(tc.input), func(t *testing.T) {
			if err := From(tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("converted %v: %v", tc.input, err)
			}

		})
	}
	if !errors.Is(From(unavailable), unavailable) {
		t.Fatal("underlying storage cause lost")
	}
}
