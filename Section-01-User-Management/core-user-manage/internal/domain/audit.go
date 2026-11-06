package domain

import (
	"context"
	"time"
)

type FieldChange struct {
	Before *string
	After  *string
}

type AuditContext struct {
	ActorType     string
	ActorID       string
	CallerService string
	RequestID     string
	TraceID       string
}

type UserAudit struct {
	ID         string
	UserID     string
	Action     string
	OccurredAt time.Time
	Actor      AuditContext
	Changes    map[string]FieldChange
}

type auditKey struct{}

func WithAuditContext(ctx context.Context, actor AuditContext) context.Context {
	return context.WithValue(ctx, auditKey{}, actor)
}

func AuditActor(ctx context.Context) AuditContext {
	actor, _ := ctx.Value(auditKey{}).(AuditContext)
	return actor
}
