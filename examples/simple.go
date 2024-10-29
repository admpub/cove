package main

import (
	"fmt"
	"github.com/modfin/cove"
	"github.com/modfin/cove/examples/helper"
	"time"
)

func main() {

	// creates a sqlite cache in a temporary directory,
	//  once the cache is closed the database is removed
	//  a default TTL of 10 minutes is set
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithTTL(time.Minute*10),
	)
	helper.AssertNoErr(err)
	defer cache.Close()

	// set a key value pair in the cache
	err = cache.Set("key", []byte("value0"))
	helper.AssertNoErr(err)

	// get the value from the cache
	value, err := cache.Get("key")
	helper.AssertNoErr(err)

	fmt.Println(string(value))
	// value0
}
