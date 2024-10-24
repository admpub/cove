package cove

import (
	"bytes"
	"encoding/gob"
	"errors"
	"iter"
	"time"
)

// Of creates a typed cache facade using generics.
// encoding/gob is used for serialization and deserialization
//
// Example:
// cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose())
// assert.NoError(err)
//
// // creates a namespace that is separate from the main cache,
// // this helps to avoid key collisions and allows for easier management of keys
// // when using multiple types
// separateNamespace, err := cache.NS("my-strings")
// assert.NoError(err)
//
// stringCache := cove.Of[string](stringNamespace)
// stringCache.Set("hello", "typed world")
// fmt.Println(stringCache.Get("hello"))
// // Output: typed world <nil>
func Of[V any](cache *Cache) *TypedCache[V] {
	return &TypedCache[V]{
		cache: cache,
	}
}

type TypedCache[V any] struct {
	cache *Cache
}

func (t *TypedCache[V]) decode(b []byte) (V, error) {
	var value V
	bb := bytes.NewBuffer(b)
	dec := gob.NewDecoder(bb)
	err := dec.Decode(&value)
	return value, err
}
func (t *TypedCache[V]) encode(v V) ([]byte, error) {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(v)
	return bb.Bytes(), err
}

// Range returns all key value pairs in the range [from, to]
func (t *TypedCache[V]) Range(from string, to string) ([]KVt[V], error) {
	kv, err := t.cache.Range(from, to)
	if err != nil {
		return nil, err
	}

	var kvs []KVt[V]
	for _, v := range kv {
		value, err := t.decode(v.V)
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, KVt[V]{K: v.K, V: value})
	}

	return kvs, err
}

// Keys returns all keys in the range [from, to]
func (t *TypedCache[V]) Keys(from string, to string) (keys []string, err error) {
	return t.cache.Keys(from, to)
}

// Values returns all values in the range [from, to]
func (t *TypedCache[V]) Values(from string, to string) (values []V, err error) {
	vals, err := t.cache.Values(from, to)
	if err != nil {
		return nil, err
	}
	for _, v := range vals {
		value, err := t.decode(v)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// ItrRange returns a k/v iterator for the range of keys [from, to]
//
// WARNING
// Since iterators don't really have any way of communication errors
// the Con is that errors are dropped when using iterators.
// the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)
func (t *TypedCache[V]) ItrRange(from string, to string) iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		t.cache.ItrRange(from, to)(func(k string, v []byte) bool {
			value, err := t.decode(v)
			if err != nil {
				return false
			}
			return yield(k, value)
		})
	}
}

// ItrKeys returns a key iterator for the range of keys [from, to]
//
// WARNING
// Since iterators don't really have any way of communication errors
// the Con is that errors are dropped when using iterators.
// the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)
func (t *TypedCache[V]) ItrKeys(from string, to string) iter.Seq[string] {
	return t.cache.ItrKeys(from, to)
}

// ItrValues returns a value iterator for the range of keys [from, to]
//
// WARNING
// Since iterators don't really have any way of communication errors
// the Con is that errors are dropped when using iterators.
// the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)
func (t *TypedCache[V]) ItrValues(from string, to string) iter.Seq[V] {
	return func(yield func(V) bool) {
		t.cache.ItrValues(from, to)(func(v []byte) bool {
			value, err := t.decode(v)
			if err != nil {
				return false
			}
			return yield(value)
		})
	}
}

// Get returns the value for the key
func (t *TypedCache[V]) Get(key string) (V, error) {
	var z V
	b, err := t.cache.Get(key)
	if err != nil {
		return z, err
	}
	value, err := t.decode(b)
	if err != nil {
		return z, err
	}
	return value, err
}

// Set sets the value for the key with the default TTL
func (t *TypedCache[V]) Set(key string, value V) error {
	return t.SetTTL(key, value, t.cache.ttl)
}

// SetTTL sets the value for the key with the specified TTL
func (t *TypedCache[V]) SetTTL(key string, value V, ttl time.Duration) error {
	b, err := t.encode(value)
	if err != nil {
		return err
	}
	return t.cache.SetTTL(key, b, ttl)
}

// GetOr returns the value for the key, if the key does not exist it will call the getter function to get the value
func (t *TypedCache[V]) GetOr(key string, getter func(key string) (V, error)) (V, error) {
	b, err := t.cache.GetOr(key, func(key string) ([]byte, error) {
		value, err := getter(key)
		if err != nil {
			return nil, err
		}
		return t.encode(value)
	})
	var value V
	if err != nil {
		return value, err
	}
	return t.decode(b)
}

// BatchSet sets a batch of key/value pairs in the cache
// the BatchSet will take place in one transaction, but split up into sub-batches of MAX_PARAMS/3 size, ie 999/3 = 333,
// in order to have the BatchSet be atomic. If one key fails to set, the whole batch will fail.
// Prefer batches less then MAX_PARAMS
func (t *TypedCache[V]) BatchSet(ziped []KVt[V]) error {
	var rows []KV
	for _, z := range ziped {
		b, err := t.encode(z.V)
		if err != nil {
			return err
		}
		rows = append(rows, KV{K: z.K, V: b})
	}
	return t.cache.BatchSet(rows)
}

// BatchGet retrieves a batch of keys from the cache
// the BatchGet will take place in one transaction, but split up into sub-batches of MAX_PARAMS size, ie 999,
// in order to have the BatchGet be atomic. If one key fails to fetched, the whole batch will fail.
// Prefer batches less then MAX_PARAMS
func (t *TypedCache[V]) BatchGet(keys []string) ([]KVt[V], error) {
	kvs, err := t.cache.BatchGet(keys)
	if err != nil {
		return nil, err
	}
	var zipped []KVt[V]
	for _, v := range kvs {
		value, err := t.decode(v.V)
		if err != nil {
			return nil, err
		}
		zipped = append(zipped, KVt[V]{K: v.K, V: value})
	}
	return zipped, nil
}

// BatchEvict evicts a batch of keys from the cache
// if onEvict is set, it will be called for each key
// the eviction will take place in one transaction, but split up into bacthes of MAX_PARAMS, ie 999,
// in order to have the eviction be atomic. If one key fails to evict, the whole batch will fail.
// Prefer batches less then MAX_PARAMS
func (t *TypedCache[V]) BatchEvict(keys []string) ([]KVt[V], error) {
	kvs, err := t.cache.BatchEvict(keys)
	if err != nil {
		return nil, err
	}
	var zipped []KVt[V]
	for _, v := range kvs {
		value, err := t.decode(v.V)
		if err != nil {
			return nil, err
		}
		zipped = append(zipped, KVt[V]{K: v.K, V: value})
	}
	return zipped, nil
}

// EvictAll evicts all keys in the cache
// onEvict will not be called
func (t *TypedCache[V]) EvictAll() (int, error) {
	return t.cache.EvictAll()
}

// Evict evicts a key from the cache
// if onEvict is set, it will be called for key
func (t *TypedCache[V]) Evict(key string) (KVt[V], error) {
	var res KVt[V]
	kv, err := t.cache.Evict(key)

	if errors.Is(err, NotFound) {
		return res, err
	}
	if err != nil && !errors.Is(err, NotFound) {
		return res, err
	}

	bb := bytes.NewBuffer(kv.V)
	dec := gob.NewDecoder(bb)
	err = dec.Decode(&res.V)
	return res, err
}

// Raw returns the underlying untyped cache
func (t *TypedCache[V]) Raw() *Cache {
	return t.cache
}
