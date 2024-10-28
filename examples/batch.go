package main

import (
	"fmt"
	"github.com/modfin/cove"
)

func assertNoErr(err error) {
	if err != nil {
		panic(err)
	}
}

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
	assertNoErr(err)
	defer cache.Close()

	err = cache.BatchSet([]cove.KV[[]byte]{
		{K: "key1", V: []byte("val1")},
		{K: "key2", V: []byte("val2")},
	})
	assertNoErr(err)

	kvs, err := cache.BatchGet([]string{"key1", "key2", "key3"})
	assertNoErr(err)

	for _, kv := range kvs {
		fmt.Println(kv.K, "-", string(kv.V))
		// output:
		//  key1 - val1
		//  key2 - val2
	}

	evicted, err := cache.BatchEvict([]string{"key1", "key2", "key3"})
	assertNoErr(err)
	// output:
	//  Callback, key key1 was evicted
	//  Callback, key key2 was evicted

	for _, kv := range evicted {
		fmt.Println("Evicted,", kv.K, "-", string(kv.V))
		// output:
		//  Evicted, key1 - val1
		//  Evicted, key2 - val2
	}
}
