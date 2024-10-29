package main

import (
	"errors"
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

	// A key that does not exist will return cove.NotFound error
	_, err = cache.Get("key")
	fmt.Println("err == cove.NotFound:", err == cove.NotFound)
	// err == cove.NotFound: true
	fmt.Println("errors.Is(err, cove.NotFound):", errors.Is(err, cove.NotFound))
	// errors.Is(err, cove.NotFound): true

	// A nice pattern to use to split the error handling from the logic of hit or not might be the following
	_, err = cache.Get("key")
	hit, err := cove.Hit(err) // false, nil
	if err != nil {
		panic(err) // something went wrong with the sqlite database
	}
	if !hit {
		fmt.Println("key miss")
		// key miss
	}

	// Or the opposite for convenience using Mis

	// A nice pattern to use to split the error handling from the logic of hit or not might be the following
	_, err = cache.Get("key")
	miss, err := cove.Miss(err) // false, nil
	if err != nil {
		panic(err) // something went wrong with the sqlite database
	}
	if miss {
		fmt.Println("key miss")
		// key miss
	}

}
