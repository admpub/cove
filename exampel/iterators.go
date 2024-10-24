package main

import (
	"fmt"
	"github.com/modfin/lcache"
	"time"
)

func assertNoErr(err error) {
	if err != nil {
		panic(err)
	}
}

// WARNING
// Since iterators don't really have any way of communication errors
// the Con is that errors are dropped when using iterators.
// the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)

func main() {

	// creates a sqlite cache in a temporary directory,
	//  once the cache is closed the database is removed
	//  a default TTL of 10 minutes is set
	cache, err := lcache.New(lcache.URITemp(),
		lcache.DBRemoveOnClose(),
		lcache.WithTTL(time.Minute*10),
	)
	assertNoErr(err)
	defer cache.Close()

	// set a key value pairs in the cache
	for i := 0; i < 100; i++ {
		err = cache.Set(fmt.Sprintf("key%d", i), []byte(fmt.Sprintf("value%d", i)))
		assertNoErr(err)
	}

	// KV iterator
	for k, v := range cache.ItrRange("key97", lcache.RANGE_MAX) {
		fmt.Println(k, string(v))
		//key97 value97
		//key98 value98
		//key99 value99
	}

	// Key iterator
	for key := range cache.ItrKeys(lcache.RANGE_MIN, "key1") {
		fmt.Println(key)
		//key0
		//key1
	}

	// Value iterator
	for value := range cache.ItrValues(lcache.RANGE_MIN, "key1") {
		fmt.Println(string(value))
		//value0
		//value1
	}
}
