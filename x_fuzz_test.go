package cove_test

import (
	"fmt"
	"github.com/modfin/cove"
	"github.com/stretchr/testify/assert"
	"testing"
)

// Since we are stepping on eachother with keys, use parallel=1
// go test -v -fuzz=. -run="^#" -parallel=1 -fuzztime 30s
func FuzzGetSet(f *testing.F) {

	testcases := []cove.KV[string]{
		{K: "key", V: "value"},
		{K: "1", V: "2"},
		{K: "2", V: ""},
		{K: "", V: ""},
		{K: "", V: "some value"},
		{K: "SELECT 1", V: "; DROP TABLE users;"},
		{K: "; DROP TABLE users;", V: "sleep 10;"},
		{K: "", V: string([]byte{0xff, 0x7f})},
		{K: string(rune(0)), V: string([]byte{0})},
		{K: string(rune(255)), V: string([]byte{255})},
		{K: "a", V: "1\x12\xd78{"},
		{K: "b", V: "\xff\x7f"},
		{"\xff\x7f\xff\xff\xaa\b", "10"},
	}
	for _, tc := range testcases {
		f.Add(tc.K, []byte(tc.V)) // Use f.Add to provide a seed corpus
	}
	cache, err := cove.New(cove.URITemp(), cove.DBRemoveOnClose())
	assert.NoError(f, err)

	f.Fuzz(func(t *testing.T, key string, val []byte) {
		fmt.Println("fuzz k:", key, "v:", string(val))

		err := cache.Set(key, val)
		assert.NoError(t, err)
		//
		v, err := cache.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, val, v)

		kv, err := cache.Evict(key)
		assert.NoError(t, err)
		assert.Equal(t, key, kv.K)
		assert.Equal(t, val, kv.V)

		v, err = cache.Get(key)
		assert.Error(t, err)
		assert.Equal(t, err, cove.NotFound)
	})
}
