package mongorepository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	mongoerrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors/mongodb"
)

var _ domain.TransactionManager = (*Transactions)(nil)

type Transactions struct {
	client *mongo.Client
}

func NewTransactions(client *mongo.Client) *Transactions {

	return &Transactions{
		client: client,
	}
}

func (t *Transactions) WithinTransaction(ctx context.Context, run func(context.Context) error) error {
	session, err := t.client.StartSession()
	if err != nil {
		return mongoerrors.From(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		session.EndSession(cleanup)
	}()
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {

		return nil, run(tx)
	}, options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()))
	// Preserve semantic callback errors; driver errors are normalized at their repository boundary.
	return err
}
