package lcache

import (
	"bytes"
	"encoding/gob"
	"time"
)

type TypedKV[V any] struct {
	Key   string
	Value V
}

type Typed[V any] struct {
	cache *Cache
}

func Of[V any](cache *Cache) *Typed[V] {
	return &Typed[V]{
		cache: cache,
	}
}
func (t *Typed[V]) decode(b []byte) (V, error) {
	var value V
	bb := bytes.NewBuffer(b)
	dec := gob.NewDecoder(bb)
	err := dec.Decode(&value)
	return value, err
}
func (t *Typed[V]) encode(v V) ([]byte, error) {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(v)
	return bb.Bytes(), err
}

func (t *Typed[V]) Range(from string, to string) ([]TypedKV[V], error) {
	kv, err := t.cache.Range(from, to)
	if err != nil {
		return nil, err
	}

	var kvs []TypedKV[V]
	for _, v := range kv {
		value, err := t.decode(v.Value)
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, TypedKV[V]{Key: v.Key, Value: value})
	}

	return kvs, err
}

func (t *Typed[V]) Get(key string) (V, error) {
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

func (t *Typed[V]) Set(key string, value V) error {
	return t.SetTTL(key, value, t.cache.ttl)
}
func (t *Typed[V]) SetTTL(key string, value V, ttl time.Duration) error {
	b, err := t.encode(value)
	if err != nil {
		return err
	}
	return t.cache.SetTTL(key, b, ttl)
}

func (t *Typed[V]) GetOrSet(key string, getter func(key string) (V, error)) (V, error) {
	b, err := t.cache.GetOrSet(key, func(key string) ([]byte, error) {
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

func (t *Typed[V]) EvictAll() (int, error) {
	return t.cache.EvictAll()
}
func (t *Typed[V]) Evict(key string) (V, error) {
	var value V
	pre, found, err := t.cache.Evict(key)
	if err != nil {
		return value, err
	}
	if !found {
		return value, nil
	}

	bb := bytes.NewBuffer(pre)
	dec := gob.NewDecoder(bb)
	err = dec.Decode(&value)
	return value, err
}

func (t *Typed[V]) Raw() *Cache {
	return t.cache
}
