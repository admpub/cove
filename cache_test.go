package lcache_test

import (
	"math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/modfin/lcache"
	"github.com/stretchr/testify/assert"
)

func TestNewCacheWithTempUri(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	defer cache.DeleteStore()
	defer cache.Close()
}

func TestCacheSetAndGet(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	err = cache.Set(key, value)
	assert.NoError(t, err)

	retrievedValue, err := cache.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestCacheGetOrSet(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	retrievedValue, err := cache.GetOrSet(key, func(k string) ([]byte, error) {
		return value, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func BenchmarkCacheSet(b *testing.B) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(b, err)
	defer cache.DeleteStore()
	defer cache.Close()

	value := []byte("1")

	var i int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ii := atomic.AddInt64(&i, 1)
			k := strconv.Itoa(int(ii))
			err = cache.Set(k, value)
			assert.NoError(b, err)
		}
	})
}

func BenchmarkCacheGet(b *testing.B) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(b, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "1"
	value := []byte("1")
	err = cache.Set(key, value)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err = cache.Get(key)
			assert.NoError(b, err)
		}

	})
}

func BenchmarkCacheLargeGet(b *testing.B) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(b, err)
	defer cache.DeleteStore()
	defer cache.Close()

	start := time.Now()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		value := []byte("1")
		err = cache.Set(key, value)
		assert.NoError(b, err)
	}
	elapsed := time.Since(start)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := strconv.Itoa(int(rand.Float64() * float64(b.N)))
			_, err = cache.Get(key)
			assert.NoError(b, err)

		}

	})
	b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "insert-ns/op")

}

func TestCacheGetOrSetParallel(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	start := make(chan struct{})

	wg := sync.WaitGroup{}

	do := func() {
		<-start
		retrievedValue, err := cache.GetOrSet(key, func(k string) ([]byte, error) {
			return value, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, value, retrievedValue)
		wg.Done()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go do()
	}
	time.Sleep(1 * time.Second)
	close(start)
	wg.Wait()
}

func TestCacheGetOrSetParallelMem(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	start := make(chan struct{})

	wg := sync.WaitGroup{}

	do := func() {
		<-start
		retrievedValue, err := cache.GetOrSet(key, func(k string) ([]byte, error) {
			return value, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, value, retrievedValue)
		wg.Done()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go do()
	}
	time.Sleep(1 * time.Second)
	close(start)
	wg.Wait()
}

func TestCacheEvict(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	err = cache.Set(key, value)
	assert.NoError(t, err)

	prevValue, found, err := cache.Evict(key)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, value, prevValue)

	_, err = cache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, lcache.NotFound, err)
}

func TestCacheSetTTL(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")
	ttl := 1 * time.Second

	err = cache.SetTTL(key, value, ttl)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = cache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, lcache.NotFound, err)
}

func TestCacheRange(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	// Set some key-value pairs in the cache
	pairs := []lcache.KV{
		{"key1", []byte("value1")},
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
		{"key5", []byte("value5")},
		{"key6", []byte("value6")},
	}

	exp := []lcache.KV{
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
	}

	for _, pair := range pairs {
		err = cache.Set(pair.Key, pair.Value)
		assert.NoError(t, err)
	}

	// Test the Range function
	kvs, err := cache.Range("key2", "key4")
	assert.NoError(t, err)
	assert.Equal(t, exp, kvs)
}

func TestCacheIter(t *testing.T) {
	cache, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer cache.DeleteStore()
	defer cache.Close()

	// Set some key-value pairs in the cache
	pairs := []lcache.KV{
		{"key1", []byte("value1")},
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
		{"key6", []byte("value5")},
		{"key7", []byte("value6")},
	}

	exp := []lcache.KV{
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
	}

	for _, pair := range pairs {
		err = cache.Set(pair.Key, pair.Value)
		assert.NoError(t, err)
	}

	var res []lcache.KV

	for k, v := range cache.Iter("key2", "key4") {

		res = append(res, lcache.KV{
			Key:   k,
			Value: v,
		})
	}

	assert.Equal(t, exp, res)

	var keys []string
	for key := range cache.Keys("key2", "key5") {
		keys = append(keys, key)
	}
	assert.Equal(t, []string{"key2", "key3:and:more", "key4"}, keys)

	var vals []string
	for val := range cache.Values("key2", "key5") {
		vals = append(vals, string(val))
	}
	assert.Equal(t, []string{"value2", "value3", "value4"}, vals)

}

func TestNS(t *testing.T) {
	c1, err := lcache.New(lcache.TempUri())
	assert.NoError(t, err)
	defer c1.DeleteStore()
	defer c1.Close()

	c1.Set("key1", []byte("ns1"))
	c2, err := c1.NS("ns2")
	assert.NoError(t, err)

	c2.Set("key1", []byte("ns2"))

	v, err := c1.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("ns1"), v)

	v, err = c2.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("ns2"), v)

	c11, err := c2.NS(lcache.NS_DEFAULT)
	assert.NoError(t, err)
	v, err = c11.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("ns1"), v)

	_, found, err := c11.Evict("key1")
	assert.NoError(t, err)
	assert.True(t, found)

	_, found, err = c1.Evict("key1")
	assert.NoError(t, err)
	assert.False(t, found)

}
