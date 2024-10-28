package cove_test

import (
	"fmt"
	"github.com/modfin/cove"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func init() {
	//slog.SetLogLoggerLevel(slog.LevelDebug)
}

func TestNewCacheWithTempUri(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	defer cache.Close()
}

func TestCacheSetAndGet(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	err = cache.Set(key, value)
	assert.NoError(t, err)

	retrievedValue, err := cache.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestCacheSetAndGetSize(t *testing.T) {
	do := func(size int) func(t *testing.T) {
		return func(t *testing.T) {
			cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
			assert.NoError(t, err)
			defer cache.Close()

			key := "test-key"

			value := make([]byte, size)
			for i := 0; i < size; i++ {
				value[i] = byte(i)
			}

			err = cache.Set(key, value)
			assert.NoError(t, err)

			retrievedValue, err := cache.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, value, retrievedValue)

		}
	}

	t.Run("__1_B", do(1))
	t.Run("__1_kB", do(1_000))
	t.Run("_10_kB", do(10_000))
	t.Run("100_kB", do(100_000))
	t.Run("__1_MB", do(1_000_000))
	t.Run("_10_MB", do(10_000_000))
	//t.Run("100_MB", do(100_000_000))
	//t.Run("__1_GB", do(1_000_000_000-100)) // Maximum blob size in SQLite is 1_000_000_000 aka 1GB (probably need some bytes for metadata)

}

func TestCacheSetGetEmpty(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte{}

	err = cache.Set(key, value)
	assert.NoError(t, err)

	retrievedValue, err := cache.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestCacheSetGetNil(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"

	err = cache.Set(key, nil)
	assert.NoError(t, err)

	exp := []byte{}
	retrievedValue, err := cache.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, exp, retrievedValue)
}

func TestCacheGetOrSet(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	retrievedValue, err := cache.GetOr(key, func(k string) ([]byte, error) {
		return value, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestCacheGetOrSetParallel(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	start := make(chan struct{})

	wg := sync.WaitGroup{}

	do := func() {
		<-start
		retrievedValue, err := cache.GetOr(key, func(k string) ([]byte, error) {
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
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	start := make(chan struct{})

	wg := sync.WaitGroup{}

	do := func() {
		<-start
		retrievedValue, err := cache.GetOr(key, func(k string) ([]byte, error) {
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
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")

	err = cache.Set(key, value)
	assert.NoError(t, err)

	prevValue, err := cache.Evict(key)
	assert.NoError(t, err)
	assert.Equal(t, value, prevValue.V)

	_, err = cache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, cove.NotFound, err)
}

func TestCacheSetTTL(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	key := "test-key"
	value := []byte("test-value")
	ttl := 1 * time.Second

	err = cache.SetTTL(key, value, ttl)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = cache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, cove.NotFound, err)
}

func TestCache_Range(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	// Set some values
	err = cache.Set("key1", []byte("value1"))
	assert.NoError(t, err)
	err = cache.Set("key2", []byte("value2"))
	assert.NoError(t, err)
	err = cache.Set("key3", []byte("value3"))
	assert.NoError(t, err)

	// Test ItrRange
	kv, err := cache.Range(cove.RANGE_MIN, cove.RANGE_MAX)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(kv))
	assert.Equal(t, "value1", string(kv[0].V))
	assert.Equal(t, "value2", string(kv[1].V))
	assert.Equal(t, "value3", string(kv[2].V))
}

func TestCache_Keys(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	// Set some values
	err = cache.Set("key1", []byte("value1"))
	assert.NoError(t, err)
	err = cache.Set("key2", []byte("value2"))
	assert.NoError(t, err)

	// Test ItrKeys
	keys, err := cache.Keys(cove.RANGE_MIN, cove.RANGE_MAX)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(keys))
	assert.Equal(t, "key1", keys[0])
	assert.Equal(t, "key2", keys[1])
}

func TestCache_Values(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	// Set some values
	err = cache.Set("key1", []byte("value1"))
	assert.NoError(t, err)
	err = cache.Set("key2", []byte("value2"))
	assert.NoError(t, err)

	// Test ItrValues
	values, err := cache.Values("key1", "key2")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(values))
	assert.Equal(t, "value1", string(values[0]))
	assert.Equal(t, "value2", string(values[1]))
}

func TestCacheRange(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	// Set some key-value pairs in the cache
	pairs := []cove.KV[[]byte]{
		{"key1", []byte("value1")},
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
		{"key5", []byte("value5")},
		{"key6", []byte("value6")},
	}

	exp := []cove.KV[[]byte]{
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
	}

	for _, pair := range pairs {
		err = cache.Set(pair.K, pair.V)
		assert.NoError(t, err)
	}

	// Test the ItrRange function
	kvs, err := cache.Range("key2", "key4")

	assert.NoError(t, err)

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].K < kvs[j].K
	})
	sort.Slice(exp, func(i, j int) bool {
		return exp[i].K < exp[j].K
	})

	assert.Equal(t, exp, kvs)
}

func TestCacheIter(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	// Set some key-value pairs in the cache
	pairs := []cove.KV[[]byte]{
		{"key1", []byte("value1")},
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
		{"key6", []byte("value5")},
		{"key7", []byte("value6")},
	}

	exp := []cove.KV[[]byte]{
		{"key2", []byte("value2")},
		{"key3:and:more", []byte("value3")},
		{"key4", []byte("value4")},
	}

	for _, pair := range pairs {
		err = cache.Set(pair.K, pair.V)
		assert.NoError(t, err)
	}

	var res []cove.KV[[]byte]

	for k, v := range cache.ItrRange("key2", "key4") {
		res = append(res, cove.KV[[]byte]{
			K: k,
			V: v,
		})
	}

	assert.Equal(t, exp, res)

	var keys []string
	for key := range cache.ItrKeys("key2", "key5") {
		keys = append(keys, key)
	}
	assert.Equal(t, []string{"key2", "key3:and:more", "key4"}, keys)

	var vals []string
	for val := range cache.ItrValues("key2", "key5") {
		vals = append(vals, string(val))
	}
	assert.Equal(t, []string{"value2", "value3", "value4"}, vals)

}

func TestNS(t *testing.T) {
	c1, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
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

	c11, err := c2.NS(cove.NS_DEFAULT)
	assert.NoError(t, err)
	v, err = c11.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("ns1"), v)

	_, err = c11.Evict("key1")
	hit, err := cove.Hit(err)
	assert.NoError(t, err)
	assert.True(t, hit)

	_, err = c1.Evict("key1")
	hit, err = cove.Hit(err)
	assert.NoError(t, err)
	assert.False(t, hit)

}

func TestCacheBatchSetSizes(t *testing.T) {

	do := func(itre int) func(t *testing.T) {
		return func(t *testing.T) {
			cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
			assert.NoError(t, err)
			defer cache.Close()

			var rows []cove.KV[[]byte]
			for i := 0; i < itre; i++ {
				rows = append(rows, cove.KV[[]byte]{K: strconv.Itoa(i), V: []byte(fmt.Sprintf("value_%d", i))})
			}

			err = cache.BatchSet(rows)
			assert.NoError(t, err)

			for _, row := range rows {
				retrievedValue, err := cache.Get(row.K)
				assert.NoError(t, err)
				assert.Equal(t, row.V, retrievedValue)
			}
		}
	}

	t.Run("10", do(10))
	t.Run("11", do(11))
	t.Run("100", do(100))
	t.Run("101", do(101))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3-1), do(cove.MAX_PARAMS/3-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3), do(cove.MAX_PARAMS/3))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3+1), do(cove.MAX_PARAMS/3+1))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS-1), do(cove.MAX_PARAMS-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS), do(cove.MAX_PARAMS))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS+1), do(cove.MAX_PARAMS+1))

	t.Run("1_013", do(1013))
	t.Run("10_000", do(10000))
	t.Run("10_007", do(10007))

}

func TestCacheBatchGetSizes(t *testing.T) {

	do := func(itre int) func(t *testing.T) {
		return func(t *testing.T) {
			cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
			assert.NoError(t, err)
			defer cache.Close()

			var keys []string
			var rows []cove.KV[[]byte]
			for i := 0; i < itre; i++ {
				kv := cove.KV[[]byte]{K: strconv.Itoa(i), V: []byte(fmt.Sprintf("value_%d", i))}
				rows = append(rows, kv)
				keys = append(keys, kv.K)
				err = cache.Set(kv.K, kv.V)
				assert.NoError(t, err)
			}

			res, err := cache.BatchGet(keys)
			assert.NoError(t, err)

			sort.Slice(res, func(i, j int) bool {
				return res[i].K < res[j].K
			})
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].K < rows[j].K
			})
			//sort.Strings(keys)

			assert.Equal(t, len(rows), len(res))
			assert.Equal(t, len(rows), len(keys))

			for i, row := range rows {
				assert.NoError(t, err)
				assert.Equal(t, string(row.K), string(res[i].K))
				assert.Equal(t, string(row.V), string(res[i].V))
			}
		}
	}

	t.Run("10", do(10))
	t.Run("11", do(11))
	t.Run("100", do(100))
	t.Run("101", do(101))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3-1), do(cove.MAX_PARAMS/3-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3), do(cove.MAX_PARAMS/3))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3+1), do(cove.MAX_PARAMS/3+1))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS-1), do(cove.MAX_PARAMS-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS), do(cove.MAX_PARAMS))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS+1), do(cove.MAX_PARAMS+1))

	t.Run("1_013", do(1013))
	t.Run("10_000", do(10000))
	t.Run("10_007", do(10007))

}

func TestCacheBatchEvictSizes(t *testing.T) {

	do := func(itre int) func(t *testing.T) {
		return func(t *testing.T) {

			var evicted []cove.KV[[]byte]
			var mu sync.Mutex
			wgEvicted := sync.WaitGroup{}
			wgEvicted.Add(itre)

			cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()),
				cove.WithEvictCallback(func(k string, v []byte) {
					mu.Lock()
					defer mu.Unlock()
					defer wgEvicted.Done()
					evicted = append(evicted, cove.KV[[]byte]{K: k, V: v})
				}),
			)
			assert.NoError(t, err)
			defer cache.Close()

			var keys []string
			var rows []cove.KV[[]byte]
			for i := 0; i < itre; i++ {
				kv := cove.KV[[]byte]{K: strconv.Itoa(i), V: []byte(fmt.Sprintf("value_%d", i))}
				rows = append(rows, kv)
				keys = append(keys, kv.K)
				err = cache.Set(kv.K, kv.V)
				assert.NoError(t, err)
			}

			res, err := cache.BatchEvict(keys)
			assert.NoError(t, err)

			sort.Slice(res, func(i, j int) bool {
				return res[i].K < res[j].K
			})
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].K < rows[j].K
			})

			assert.Equal(t, len(rows), len(res))
			assert.Equal(t, len(rows), len(keys))

			for i, row := range rows {
				assert.Equal(t, string(row.K), string(res[i].K))
				assert.Equal(t, string(row.V), string(res[i].V))

				_, err = cache.Get(row.K)
				assert.Error(t, err)
				assert.Equal(t, cove.NotFound, err)
			}

			wgEvicted.Wait()
			sort.Slice(evicted, func(i, j int) bool {
				return evicted[i].K < evicted[j].K
			})
			assert.Equal(t, rows, evicted)

		}
	}

	t.Run("10", do(10))
	t.Run("11", do(11))
	t.Run("100", do(100))
	t.Run("101", do(101))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3-1), do(cove.MAX_PARAMS/3-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3), do(cove.MAX_PARAMS/3))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS/3+1), do(cove.MAX_PARAMS/3+1))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS-1), do(cove.MAX_PARAMS-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS), do(cove.MAX_PARAMS))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS+1), do(cove.MAX_PARAMS+1))

	t.Run("1_013", do(1013))
	t.Run("10_000", do(10000))
	t.Run("10_007", do(10007))

}

func TestCacheBatchSet(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	rows := []cove.KV[[]byte]{
		{K: "key1", V: []byte("value1")},
		{K: "key2", V: []byte("value2")},
		{K: "key3", V: []byte("value3")},
	}

	err = cache.BatchSet(rows)
	assert.NoError(t, err)

	for _, row := range rows {
		retrievedValue, err := cache.Get(row.K)
		assert.NoError(t, err)
		assert.Equal(t, row.V, retrievedValue)
	}
}

func TestCacheBatchGet(t *testing.T) {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
	assert.NoError(t, err)
	defer cache.Close()

	rows := []cove.KV[[]byte]{
		{K: "key1", V: []byte("value1")},
		{K: "key2", V: []byte("value2")},
		{K: "key3", V: []byte("value3")},
	}

	err = cache.BatchSet(rows)
	assert.NoError(t, err)

	keys := []string{"key1", "key2", "key3", "key4"}
	retrievedRows, err := cache.BatchGet(keys)
	assert.NoError(t, err)

	sort.Slice(retrievedRows, func(i, j int) bool {
		return retrievedRows[i].K < retrievedRows[j].K
	})

	assert.Equal(t, rows, retrievedRows)
}

func TestCacheVacuum(t *testing.T) {

	do := func(itre int) func(t *testing.T) {
		return func(t *testing.T) {

			var evicted []cove.KV[[]byte]
			var mu sync.Mutex
			wgEvicted := sync.WaitGroup{}
			wgEvicted.Add(itre)

			ttl := 500 * time.Millisecond

			cache, err := cove.New(
				cove.URITemp(),
				cove.DBRemoveOnClose(),
				cove.WithLogger(slog.Default()),
				cove.WithTTL(ttl),
				//cove.WithLogger(slog.Default()),
				cove.WithVacuum(cove.Vacuum(100*time.Millisecond, 1000)),
				cove.WithEvictCallback(func(k string, v []byte) {
					mu.Lock()
					defer mu.Unlock()
					defer wgEvicted.Done()
					evicted = append(evicted, cove.KV[[]byte]{K: k, V: v})
				}),
			)
			assert.NoError(t, err)
			defer cache.Close()

			var rows []cove.KV[[]byte]
			for i := 0; i < itre; i++ {
				kv := cove.KV[[]byte]{K: strconv.Itoa(i), V: []byte(fmt.Sprintf("value_%d", i))}
				rows = append(rows, kv)
			}

			const chunkSize = 100
			for i := 0; i < len(rows); i += chunkSize {
				end := i + chunkSize
				if end > len(rows) {
					end = len(rows)
				}
				err = cache.BatchSet(rows[i:end])
				assert.NoError(t, err)
			}

			wgEvicted.Wait()
			sort.Slice(evicted, func(i, j int) bool {
				return evicted[i].K < evicted[j].K
			})
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].K < rows[j].K
			})
			assert.Equal(t, rows, evicted)
		}
	}

	t.Run("10", do(10))
	t.Run("1_000", do(1_000))
	t.Run("100_000", do(100_000))
	//t.Run("1_000_000", do(1_000_000))
}

func TestSpecific1(t *testing.T) { // from fuzz
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose())
	assert.NoError(t, err)
	defer cache.Close()

	vv1 := []byte{0x31, 0x12, 0xd7, 0x38, 0x7b}
	vv2 := []byte{0x32, 0xc9, 0x42, 0x47, 0x7, 0x93}
	err = cache.Set("key1", vv1)
	assert.NoError(t, err)
	err = cache.Set("key2", vv2)
	assert.NoError(t, err)

	v1, err := cache.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, vv1, v1)
	v2, err := cache.Get("key2")
	assert.NoError(t, err)
	assert.Equal(t, vv2, v2)
}
func TestSpecific2(t *testing.T) { // from fuzz
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose())
	assert.NoError(t, err)
	defer cache.Close()

	kk1 := string("\x80")
	vv1 := []byte("2")
	err = cache.Set(kk1, vv1)
	assert.NoError(t, err)

	v1, err := cache.Get(kk1)
	assert.NoError(t, err)
	assert.Equal(t, vv1, v1)

	kk1 = string(rune(0x80))
	vv1 = []byte("3")
	err = cache.Set(kk1, vv1)
	assert.NoError(t, err)

	v1, err = cache.Get(kk1)
	assert.NoError(t, err)
	assert.Equal(t, vv1, v1)

	kk1 = string([]byte{0x80})
	vv1 = []byte("4")
	err = cache.Set(kk1, vv1)
	assert.NoError(t, err)

	v1, err = cache.Get(kk1)
	assert.NoError(t, err)
	assert.Equal(t, vv1, v1)
}

func TestSpecific3(t *testing.T) { // from fuzz
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose())
	assert.NoError(t, err)
	defer cache.Close()

	kk1 := string("")
	vv1 := []byte("\xcd\x00\x14=8E\xbf\x9b\x9e\x81\x0e2\xe7\x17\x10\x94ݿ\xadbT\x92\x9c/\xa8\x8f(\xf07gl\xae\x00J)\x97'\xe5")
	err = cache.Set(kk1, vv1)
	assert.NoError(t, err)

	v1, err := cache.Get(kk1)
	assert.NoError(t, err)
	assert.Equal(t, vv1, v1)
}

//
//func BenchmarkCacheSet(b *testing.B) {
//	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
//	assert.NoError(b, err)
//	defer cache.Close()
//
//	value := []byte("1")
//
//	var i int64
//	b.ResetTimer()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			ii := atomic.AddInt64(&i, 1)
//			k := strconv.Itoa(int(ii))
//			err = cache.Set(k, value)
//			assert.NoError(b, err)
//		}
//	})
//}
//
//func BenchmarkCacheGet(b *testing.B) {
//	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), cove.WithLogger(slog.Default()))
//	assert.NoError(b, err)
//	defer cache.Close()
//
//	key := "1"
//	value := []byte("1")
//	err = cache.Set(key, value)
//	b.ResetTimer()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			_, err = cache.Get(key)
//			assert.NoError(b, err)
//		}
//
//	})
//}
//
//func BenchmarkCacheLargeSetGet(bbb *testing.B) {
//
//	bench := func(op ...cove.Op) func(*testing.B) {
//		return func(b *testing.B) {
//			cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), op..., cove.WithLogger(slog.Default()))
//			assert.NoError(b, err)
//			defer cache.Close()
//
//			start := time.Now()
//			for i := 0; i < b.N; i++ {
//				key := strconv.Itoa(i)
//				value := []byte(key)
//				err = cache.Set(key, value)
//				assert.NoError(b, err)
//			}
//			elapsed := time.Since(start)
//
//			b.ResetTimer()
//			b.RunParallel(func(pb *testing.PB) {
//				for pb.Next() {
//					key := strconv.Itoa(int(rand.Float64() * float64(b.N)))
//					_, err = cache.Get(key)
//					assert.NoError(b, err)
//
//				}
//
//			})
//			b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "insert-namespace/op")
//		}
//	}
//
//	bbb.Run("default", bench(cove.dbDefault()))
//	bbb.Run("default/op_journal", bench(cove.dbDefault(), cove.DBJournalSize(cove.OptimizedJournalSize)))
//	bbb.Run("default/sync_off", bench(cove.dbDefault(), cove.DBSyncOff()))
//	bbb.Run("default/sync_off/op_journal", bench(cove.dbDefault(), cove.DBSyncOff(), cove.DBJournalSize(cove.OptimizedJournalSize)))
//
//	//bbb.Run("optimized", bench(cove.DBFullOptimize()))
//	//bbb.Run("optimized sync off", bench(cove.DBFullOptimize(), cove.DBSyncOff()))
//
//}
//
//func BenchmarkCacheRealWorld(bbb *testing.B) {
//
//	bench := func(op ...cove.Op) func(*testing.B) {
//		return func(b *testing.B) {
//			cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose(), op..., cove.WithLogger(slog.Default()))
//			assert.NoError(b, err)
//			defer cache.Close()
//
//			done := make(chan struct{})
//			wg := sync.WaitGroup{}
//			wg2 := sync.WaitGroup{}
//			mu := sync.Mutex{}
//
//			var avgInsert float64
//			var avgInsertCount float64
//
//			var avgEvict float64
//			var avgEvictCount float64
//
//			for i := 0; i < runtime.NumCPU()/2; i++ {
//				wg.Add(1)
//				wg2.Add(1)
//				go func() {
//					defer wg2.Done()
//					wg.Done()
//					wg.Wait()
//					for i := 0; i < b.N; i++ {
//
//						select {
//						case <-done:
//							return
//						default:
//						}
//
//						key := strconv.Itoa(int(rand.Float64() * float64(b.N)))
//						value := []byte("1")
//						start := time.Now()
//						err := cache.Set(key, value)
//						elapsed := time.Since(start)
//						assert.NoError(b, err)
//
//						mu.Lock()
//
//						avgInsert = (avgInsert*avgInsertCount + float64(elapsed.Nanoseconds())) / (avgInsertCount + 1)
//						avgInsertCount++
//
//						mu.Unlock()
//
//						if i%100 == 0 {
//
//							start = time.Now()
//							_, _, err := cache.Evict(key)
//							elapsed = time.Since(start)
//							mu.Lock()
//							avgEvict = (avgEvict*avgEvictCount + float64(elapsed.Nanoseconds())) / (avgEvictCount + 1)
//							avgEvictCount++
//							mu.Unlock()
//							assert.NoError(b, err)
//						}
//					}
//
//				}()
//			}
//			wg.Wait()
//
//			b.ResetTimer()
//			b.RunParallel(func(pb *testing.PB) {
//				for pb.Next() {
//					key := strconv.Itoa(int(rand.Float64() * float64(b.N)))
//					_, err := cache.Get(key)
//					if errors.Is(cove.NotFound, err) {
//						continue
//					}
//					assert.NoError(b, err)
//				}
//			})
//			close(done)
//			wg2.Wait()
//
//			b.ReportMetric(avgInsert, "insert-namespace/op")
//			b.ReportMetric(avgEvict, "evict-namespace/op")
//		}
//
//	}
//
//	bbb.Run("default", bench(cove.dbDefault()))
//	bbb.Run("default/op_journal", bench(cove.dbDefault(), cove.DBJournalSize(cove.OptimizedJournalSize)))
//	bbb.Run("default/sync_off", bench(cove.dbDefault(), cove.DBSyncOff()))
//	bbb.Run("default/sync_off/op_journal", bench(cove.dbDefault(), cove.DBSyncOff(), cove.DBJournalSize(cove.OptimizedJournalSize)))
//
//	//bbb.Run("optimized", bench(cove.DBFullOptimize()))
//	//bbb.Run("optimized sync off", bench(cove.DBFullOptimize(), cove.DBSyncOff()))
//
//}
