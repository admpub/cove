package main

import (
	"fmt"
	"github.com/modfin/cove"
	"time"
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
		cove.WithTTL(time.Minute*10),
	)
	assertNoErr(err)
	defer cache.Close()

	// set a key value pairs in the cache
	for i := 0; i < 100; i++ {
		err = cache.Set(fmt.Sprintf("key%d", i), []byte(fmt.Sprintf("value%d", i)))
		assertNoErr(err)
	}

	// Tuple range
	kvs, err := cache.Range("key97", cove.RANGE_MAX)
	assertNoErr(err)

	for _, kv := range kvs {
		fmt.Println(kv.K, string(kv.V))
		//key97 value97
		//key98 value98
		//key99 value99
	}

	// Key range
	keys, err := cache.Keys(cove.RANGE_MIN, "key1")
	assertNoErr(err)

	for _, key := range keys {
		fmt.Println(key)
		//key0
		//key1
	}

	// Value range
	values, err := cache.Values(cove.RANGE_MIN, "key1")
	assertNoErr(err)

	for _, value := range values {
		fmt.Println(string(value))
		//value0
		//value1
	}
}
