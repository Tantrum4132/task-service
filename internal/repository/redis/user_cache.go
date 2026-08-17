package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultUserCacheTTL = 15 * time.Minute
	userCachePrefix     = "user:id:"
)

type UserCacheDecorator struct {
	next repository.UserRepository
	rdb  redis.Cmdable
	ttl  time.Duration
}

func NewUserCacheDecorator(next repository.UserRepository, rdb redis.Cmdable, ttl time.Duration) repository.UserRepository {
	if ttl <= 0 {
		ttl = DefaultUserCacheTTL
	}
	return &UserCacheDecorator{
		next: next,
		rdb:  rdb,
		ttl:  ttl,
	}
}

func (d *UserCacheDecorator) Create(ctx context.Context, exec repository.DBEngine, user *model.User) error {
	return d.next.Create(ctx, exec, user)
}

func (d *UserCacheDecorator) FindByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.User, error) {
	if id <= 0 {
		return nil, repository.ErrUserNotFound
	}

	cacheKey := fmt.Sprintf("%s%d", userCachePrefix, id)

	val, err := d.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var user model.User
		if err := json.Unmarshal([]byte(val), &user); err == nil {
			return &user, nil
		}
	}

	user, err := d.next.FindByID(ctx, exec, id)
	if err != nil {
		return nil, err
	}

	if user != nil {
		data, err := json.Marshal(user)
		if err == nil {
			_ = d.rdb.Set(ctx, cacheKey, data, d.ttl).Err()
		}
	}

	return user, nil
}

func (d *UserCacheDecorator) FindByEmail(ctx context.Context, exec repository.DBEngine, email string) (*model.User, error) {
	return d.next.FindByEmail(ctx, exec, email)
}

func (d *UserCacheDecorator) Update(ctx context.Context, exec repository.DBEngine, user *model.User) error {
	err := d.next.Update(ctx, exec, user)
	if err != nil {
		return err
	}

	if user != nil && exec == nil {
		_ = d.InvalidateUserCache(ctx, user.ID)
	}

	return nil
}

func (d *UserCacheDecorator) Delete(ctx context.Context, exec repository.DBEngine, id int64) error {
	err := d.next.Delete(ctx, exec, id)
	if err != nil {
		return err
	}

	if id != 0 && exec == nil {
		_ = d.InvalidateUserCache(ctx, id)
	}

	return nil
}

func (d *UserCacheDecorator) InvalidateUserCache(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}

	cacheKey := fmt.Sprintf("%s%d", userCachePrefix, id)
	err := d.rdb.Del(ctx, cacheKey).Err()
	if err != nil && err != redis.Nil {
		return err
	}

	return nil
}
