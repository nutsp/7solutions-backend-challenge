package redisrepository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
)

var create = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE',KEYS[2],'-inf',ARGV[4])
if redis.call('ZCARD',KEYS[2]) >= 100 then return 0 end
redis.call('HSET',KEYS[1],'uid',ARGV[1],'rh',ARGV[2],'exp',ARGV[3])
redis.call('PEXPIREAT',KEYS[1],ARGV[3])
redis.call('ZADD',KEYS[2],ARGV[3],ARGV[5])
local latest=redis.call('ZREVRANGE',KEYS[2],0,0,'WITHSCORES')
redis.call('PEXPIREAT',KEYS[2],latest[2])
return 1
`)

var rotate = redis.NewScript(`if redis.call('HGET',KEYS[1],'rh') ~= ARGV[1] then return 0 end
redis.call('HSET',KEYS[1],'rh',ARGV[2]) return 1`)

var remove = redis.NewScript(`redis.call('DEL',KEYS[1]) redis.call('ZREM',KEYS[2],ARGV[1]) return 1`)

type Sessions struct {
	client *redis.Client
}

func New(client *redis.Client) *Sessions {

	return &Sessions{
		client: client,
	}
}

func (r *Sessions) CreateSession(ctx context.Context, s domain.Session) error {
	n, err := create.Run(ctx, r.client, []string{key(s.UserID, s.ID), index(s.UserID)}, s.UserID, s.RefreshHash, s.ExpiresAt.UnixMilli(), time.Now().UnixMilli(), s.ID).Int()
	if err != nil {
		return apperrors.FromDependency("redis", err)
	}
	if n == 0 {
		return apperrors.ErrSessionLimit
	}
	return nil
}

func (r *Sessions) GetSession(ctx context.Context, uid, sid string) (domain.Session, error) {
	values, err := r.client.HMGet(ctx, key(uid, sid), "uid", "rh", "exp").Result()
	if err != nil {
		return domain.Session{}, apperrors.FromDependency("redis", err)
	}
	if len(values) != 3 || values[0] == nil || values[1] == nil || values[2] == nil {
		return domain.Session{}, apperrors.ErrNotFound
	}
	expires, err := strconv.ParseInt(fmt.Sprint(values[2]), 10, 64)
	if err != nil {
		return domain.Session{}, apperrors.FromDependency("redis", err)
	}
	return domain.Session{
		ID:          sid,
		UserID:      fmt.Sprint(values[0]),
		RefreshHash: fmt.Sprint(values[1]),
		ExpiresAt:   time.UnixMilli(expires),
	}, nil
}

func (r *Sessions) Rotate(ctx context.Context, s domain.Session, old, new string) error {
	n, err := rotate.Run(ctx, r.client, []string{key(s.UserID, s.ID)}, old, new).Int()
	if err != nil {
		return apperrors.FromDependency("redis", err)
	}
	if n != 1 {
		return apperrors.ErrUnauthorized
	}
	return nil
}

func (r *Sessions) DeleteSession(ctx context.Context, uid, sid string) error {
	return apperrors.FromDependency("redis", remove.Run(ctx, r.client, []string{key(uid, sid), index(uid)}, sid).Err())
}

func (r *Sessions) DeleteSessions(ctx context.Context, uid string) error {
	ids, err := r.client.ZRange(ctx, index(uid), 0, -1).Result()
	if err != nil {
		return apperrors.FromDependency("redis", err)
	}
	keys := []string{index(uid)}
	for _, sid := range ids {
		keys = append(keys, key(uid, sid))
	}
	return apperrors.FromDependency("redis", r.client.Del(ctx, keys...).Err())
}

func index(uid string) string {

	return "auth:{" + uid + "}:sessions"
}

func key(uid, sid string) string {

	return "auth:{" + uid + "}:session:" + sid
}
