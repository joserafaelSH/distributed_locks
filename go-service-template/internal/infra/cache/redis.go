package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg config.Config) *redis.Client {
	log.Printf("Connecting to Redis at %s", cfg.Redis.Addr)
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password})

	ctx := context.Background()

	// Ping to verify connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	return rdb
}

// LockExists reports whether a lock is currently held for the given key.
func LockExists(ctx context.Context, rdb *redis.Client, key string) (bool, error) {
	n, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("checking lock %q: %w", key, err)
	}
	return n > 0, nil
}

func LockHasOwner(ctx context.Context, rdb *redis.Client, key string) (string, error) {
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // lock does not exist
		}
		return "", fmt.Errorf("checking lock %q owner: %w", key, err)
	}
	return val, nil
}

// SetLock tries to acquire the lock for key, expiring automatically after ttl
// so a crashed holder can't block it forever. owner should identify the
// caller (worker id/hostname) so it can be verified before release. It uses
// SET NX so acquisition is atomic — no separate exists-then-set race.
// Returns true if the lock was acquired, false if it's already held.
func SetLock(ctx context.Context, rdb *redis.Client, key, owner string, ttl time.Duration) (bool, error) {
	ok, err := rdb.SetNX(ctx, key, owner, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("setting lock %q: %w", key, err)
	}
	return ok, nil
}
