package main

import (
	"fmt"
	"github.com/modfin/cove"
	"github.com/modfin/cove/examples/helper"
	"time"
)

// WARNING
// Since iterators don't really have any way of communication errors
// the Con is that errors are dropped when using iterators.
// the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)

func main() {

	// creates a sqlite cache in a temporary directory,
	//  once the cache is closed the database is removed
	//  a default TTL of 10 minutes is set
	cache, err := cove.New(cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithTTL(time.Minute*10),
	)
	helper.AssertNoErr(err)
	defer cache.Close()

	// set a key value pairs in the cache
	for i := 0; i < 100; i++ {
		err = cache.Set(fmt.Sprintf("key%d", i), []byte(fmt.Sprintf("value%d", i)))
		helper.AssertNoErr(err)
	}

	// KV iterator
	for k, v := range cache.ItrRange("key97", cove.RANGE_MAX) {
		fmt.Println(k, string(v))
		//key97 value97
		//key98 value98
		//key99 value99
	}

	// Key iterator
	for key := range cache.ItrKeys(cove.RANGE_MIN, "key1") {
		fmt.Println(key)
		//key0
		//key1
	}

	// Value iterator
	for value := range cache.ItrValues(cove.RANGE_MIN, "key1") {
		fmt.Println(string(value))
		//value0
		//value1
	}
}
