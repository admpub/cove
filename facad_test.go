package lcache_test

import (
	"testing"
	"time"

	"github.com/modfin/lcache"
	"github.com/stretchr/testify/assert"
)

func TypedCache(t *testing.T) *lcache.Typed[string] {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	return lcache.Of[string](cache)
}

type Complex struct {
	Name    string
	Ref     *Complex
	Slice   []int
	private string
	NilErr  error
}

func TypedComplexCache(t *testing.T) *lcache.Typed[Complex] {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	return lcache.Of[Complex](cache)
}

func TestGetTypedComplexCache(t *testing.T) {
	cache := TypedComplexCache(t)
	defer cache.Raw().DeleteStore()
	defer cache.Raw().Close()

	a := Complex{
		Name:   "name",
		NilErr: nil,
		Slice:  []int{1, 2, 3},
		Ref: &Complex{
			Name: "ref",
		},
		private: "private",
	}
	err := cache.Set("a", a)
	assert.NoError(t, err)

	aa, err := cache.Get("a")
	assert.NoError(t, err)
	assert.NotEqual(t, a, aa)
	a.private = ""
	assert.Equal(t, a, aa)

}

func TestGetTypedCacheValue(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().DeleteStore()
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"

	err := typedCache.Set(key, value)
	assert.NoError(t, err)

	retrievedValue, err := typedCache.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestSetTypedCacheValue(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().DeleteStore()
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"

	err := typedCache.Set(key, value)
	assert.NoError(t, err)

	retrievedValue, err := typedCache.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestSetTypedCacheValueWithTTL(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().DeleteStore()
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"
	ttl := 1 * time.Second

	err := typedCache.SetTTL(key, value, ttl)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = typedCache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, lcache.NotFound, err)
}

func TestGetOrSetTypedCacheValue(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().DeleteStore()
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"

	retrievedValue, err := typedCache.GetOrSet(key, func(key string) (string, error) {
		return value, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestEvictTypedCacheValue(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().DeleteStore()
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"

	err := typedCache.Set(key, value)
	assert.NoError(t, err)

	evictedValue, err := typedCache.Evict(key)
	assert.NoError(t, err)
	assert.Equal(t, value, evictedValue)

	_, err = typedCache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, lcache.NotFound, err)
}

func TestEvictAllTypedCacheValues(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().DeleteStore()
	defer typedCache.Raw().Close()

	key1 := "test-key1"
	value1 := "test-value1"
	key2 := "test-key2"
	value2 := "test-value2"

	err := typedCache.Set(key1, value1)
	assert.NoError(t, err)
	err = typedCache.Set(key2, value2)
	assert.NoError(t, err)

	count, err := typedCache.EvictAll()
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	_, err = typedCache.Get(key1)
	assert.Error(t, err)
	assert.Equal(t, lcache.NotFound, err)

	_, err = typedCache.Get(key2)
	assert.Error(t, err)
	assert.Equal(t, lcache.NotFound, err)
}
