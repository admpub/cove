package cove_test

import (
	"fmt"
	"github.com/modfin/cove"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TypedCache(t *testing.T) *cove.TypedCache[string] {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose())
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	return cove.Of[string](cache)
}

type Complex struct {
	Name    string
	Ref     *Complex
	Slice   []int
	private string
	NilErr  error
}

func TypedComplexCache(t *testing.T) *cove.TypedCache[Complex] {
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose())
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	return cove.Of[Complex](cache)
}

func TestGetTypedComplexCache(t *testing.T) {
	cache := TypedComplexCache(t)
	defer cache.Raw().Close()

	a := Complex{
		Name:   "name",
		NilErr: nil,
		Slice:  []int{1, 2, 3},
		Ref: &Complex{
			Name: "namespaces",
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
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"
	ttl := 1 * time.Second

	err := typedCache.SetTTL(key, value, ttl)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = typedCache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, cove.NotFound, err)
}

func TestGetOrSetTypedCacheValue(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"

	retrievedValue, err := typedCache.GetOr(key, func(key string) (string, error) {
		return value, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestEvictTypedCacheValue(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	key := "test-key"
	value := "test-value"

	err := typedCache.Set(key, value)
	assert.NoError(t, err)

	evicted, err := typedCache.Evict(key)
	assert.NoError(t, err)
	assert.Equal(t, value, evicted.Value())

	_, err = typedCache.Get(key)
	assert.Error(t, err)
	assert.Equal(t, cove.NotFound, err)
}

func TestEvictAllTypedCacheValues(t *testing.T) {
	typedCache := TypedCache(t)
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
	assert.Equal(t, cove.NotFound, err)

	_, err = typedCache.Get(key2)
	assert.Error(t, err)
	assert.Equal(t, cove.NotFound, err)
}

func TestBatchSetTypedCacheValues(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	for _, z := range ziped {
		retrievedValue, err := typedCache.Get(z.K)
		assert.NoError(t, err)
		assert.Equal(t, z.V, retrievedValue)
	}
}

func TestBatchGetTypedCacheValues(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	keys := []string{"key1", "key2"}
	zipped, err := typedCache.BatchGet(keys)
	assert.NoError(t, err)

	sort.Slice(zipped, func(i, j int) bool {
		return zipped[i].K < zipped[j].K
	})

	assert.Equal(t, ziped, zipped)
}

func TestCacheBatchEvictTypedSizes(t *testing.T) {

	do := func(itre int) func(t *testing.T) {
		return func(t *testing.T) {
			cc, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose())
			assert.NoError(t, err)
			defer cc.Close()

			typed := cove.Of[string](cc)

			var keys []string
			var rows []cove.KVt[string]
			for i := 0; i < itre; i++ {
				k := strconv.Itoa(i)
				v := fmt.Sprintf("value_%d", i)
				rows = append(rows, cove.KVt[string]{K: k, V: v})
				keys = append(keys, k)
				err = typed.Set(k, v)
				assert.NoError(t, err)
			}

			res, err := typed.BatchEvict(keys)
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
				assert.Equal(t, row.K, res[i].K)
				assert.Equal(t, row.V, res[i].V)

				_, err = typed.Get(row.K)
				assert.Error(t, err)
				assert.Equal(t, cove.NotFound, err)
			}

		}
	}

	t.Run("10", do(10))
	t.Run("11", do(11))
	t.Run("100", do(100))
	t.Run("101", do(101))

	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS-1), do(cove.MAX_PARAMS-1))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS), do(cove.MAX_PARAMS))
	t.Run(fmt.Sprintf("%d", cove.MAX_PARAMS+1), do(cove.MAX_PARAMS+1))

}

func TestTypedCacheItrRange(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
		{K: "key3", V: "value3"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	var result []cove.KVt[string]
	typedCache.ItrRange("key1", "key3")(func(k string, v string) bool {
		result = append(result, cove.KVt[string]{K: k, V: v})
		return true
	})

	sort.Slice(result, func(i, j int) bool {
		return result[i].K < result[j].K
	})

	assert.Equal(t, ziped, result)
}

func TestTypedCacheItrKeys(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
		{K: "key3", V: "value3"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	var keys []string
	typedCache.ItrKeys("key1", "key3")(func(k string) bool {
		keys = append(keys, k)
		return true
	})

	sort.Strings(keys)
	expectedKeys := []string{"key1", "key2", "key3"}
	assert.Equal(t, expectedKeys, keys)
}

func TestTypedCacheItrValues(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
		{K: "key3", V: "value3"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	var values []string
	typedCache.ItrValues("key1", "key3")(func(v string) bool {
		values = append(values, v)
		return true
	})

	sort.Strings(values)
	expectedValues := []string{"value1", "value2", "value3"}
	assert.Equal(t, expectedValues, values)
}

func TestTypedCacheValues(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
		{K: "key3", V: "value3"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	values, err := typedCache.Values("key1", cove.RANGE_MAX)
	assert.NoError(t, err)
	sort.Strings(values)
	expectedValues := []string{"value1", "value2", "value3"}
	assert.Equal(t, expectedValues, values)
}

func TestTypedCacheKeys(t *testing.T) {
	typedCache := TypedCache(t)
	defer typedCache.Raw().Close()

	ziped := []cove.KVt[string]{
		{K: "key1", V: "value1"},
		{K: "key2", V: "value2"},
		{K: "key3", V: "value3"},
	}

	err := typedCache.BatchSet(ziped)
	assert.NoError(t, err)

	values, err := typedCache.Keys("key1", cove.RANGE_MAX)
	assert.NoError(t, err)
	sort.Strings(values)
	expectedValues := []string{"key1", "key2", "key3"}
	assert.Equal(t, expectedValues, values)
}
