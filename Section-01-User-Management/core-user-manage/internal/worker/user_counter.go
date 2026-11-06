package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/requestcontext"
)

// UserCounter is the read-only application contract consumed by the statistics worker.
type UserCounter interface {
	Count(context.Context) (int64, error)
}

func Run(ctx context.Context, users UserCounter, log *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			started := time.Now()
			job, err := requestcontext.New(ctx, "")
			if err != nil {
				log.Error("worker context unavailable")
				continue
			}
			job, span := otel.Tracer("user-api/worker").Start(job, "worker.count_users")
			jobLog := logger.WithContext(job, log)
			query, cancel := context.WithTimeout(job, 3*time.Second)
			count, err := users.Count(query)
			cancel()
			if err != nil {
				span.SetStatus(codes.Error, "count users failed")
			}
			span.End()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				jobLog.WarnContext(job, "count users failed", "error", err, "duration_ms", time.Since(started).Milliseconds())
			} else {
				jobLog.InfoContext(job, "user count", "count", count, "duration_ms", time.Since(started).Milliseconds())
			}
		}
	}
}
