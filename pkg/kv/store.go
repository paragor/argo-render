package kv

import "context"

type Store interface {
	Get(ctx context.Context, key string) (any, error)
}
