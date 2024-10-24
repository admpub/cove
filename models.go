package lcache

import (
	"errors"
	"time"
)

const NS_DEFAULT = "default"
const MAX_PARAMS = 999
const MAX_BLOB_SIZE = 1_000_000_000 - 100 // 1GB - 100 bytes

const RANGE_MIN = string(byte(0))
const RANGE_MAX = string(byte(255))

// NO_TTL is a constant that represents no ttl, kind of, it is really just a very long time for a cache
const NO_TTL = time.Hour * 24 * 365 * 100 // 100 years (effectively forever)

var NotFound = errors.New("not found")

type KVt[T any] struct {
	K string
	V T
}

func (z KVt[T]) Key() string {
	return z.K
}
func (z KVt[T]) Value() T {
	return z.V
}

type KV struct {
	K string
	V []byte
}

func (z KV) Key() string {
	return z.K
}
func (z KV) Value() []byte {
	return z.V
}
