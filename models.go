package cove

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

func Zip[V any](keys []string, values []V) []KV[V] {
	size := min(len(keys), len(values))
	var res = make([]KV[V], size)

	for i := 0; i < size; i++ {
		res[i] = KV[V]{K: keys[i], V: values[i]}
	}
	return res
}

func Unzip[V any](kv []KV[V]) (keys []string, vals []V) {
	size := len(kv)
	keys = make([]string, size)
	vals = make([]V, size)

	for i, kv := range kv {
		keys[i] = kv.K
		vals[i] = kv.V
	}
	return keys, vals
}

type KV[T any] struct {
	K string
	V T
}

func (kv KV[T]) Unzip() (string, T) {
	return kv.K, kv.V
}

func (kv KV[T]) Key() string {
	return kv.K
}
func (kv KV[T]) Value() T {
	return kv.V
}
