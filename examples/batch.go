package main

import (
	"fmt"

	"github.com/admpub/cove"
	"github.com/admpub/cove/examples/helper"
)

func main() {

	// creates a sqlite cache in a temporary directory,
	//  once the cache is closed the database is removed
	//  a default TTL of 10 minutes is set
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithEvictCallback(func(key string, _ []byte) {
			fmt.Println("Callback, key", key, "was evicted")
		}),
	)
	helper.AssertNoErr(err)
	defer cache.Close()

	// Helper functions to construct []KV[[]byte] slice
	KeyValueSet := cove.Zip(
		[]string{"key1", "key2"},
		[][]byte{[]byte("val1"), []byte("val2")})

	err = cache.BatchSet(KeyValueSet)
	helper.AssertNoErr(err)

	kvs, err := cache.BatchGet([]string{"key1", "key2", "key3"})
	helper.AssertNoErr(err)

	for _, kv := range kvs {
		fmt.Println(kv.Unzip())
		// output:
		//  key1 [118 97 108 49]
		//  key2 [118 97 108 50]
	}

	evicted, err := cache.BatchEvict([]string{"key1", "key2", "key3"})
	helper.AssertNoErr(err)
	// output:
	//  Callback, key key1 was evicted
	//  Callback, key key2 was evicted

	// Helper function to unzip []KV[[]byte] to k/v slices
	evictedKeys, evictedVals := cove.Unzip(evicted)

	for i, key := range evictedKeys {
		fmt.Println("Evicted,", key, "-", string(evictedVals[i]))
		// output:
		//  Evicted, key1 - val1
		//  Evicted, key2 - val2
	}
}
