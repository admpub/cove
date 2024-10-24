package main

import (
	"errors"
	"fmt"
	"github.com/modfin/lcache"
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
	cache, err := lcache.New(
		lcache.URITemp(),
		lcache.DBRemoveOnClose(),
		lcache.WithTTL(time.Minute*10),
	)
	assertNoErr(err)
	defer cache.Close()

	// A key that does not exist will return lcache.NotFound error
	_, err = cache.Get("key")
	fmt.Println("err == lcache.NotFound:", err == lcache.NotFound)
	// err == lcache.NotFound: true
	fmt.Println("errors.Is(err, lcache.NotFound):", errors.Is(err, lcache.NotFound))
	// errors.Is(err, lcache.NotFound): true

	// A nice pattern to use to split the error handling from the logic of hit or not might be the following
	_, err = cache.Get("key")
	hit, err := lcache.Hit(err) // false, nil
	if err != nil {
		panic(err) // something went wrong with the sqlite database
	}
	if !hit {
		fmt.Println("key miss")
		// key miss
	}

	// Or the opposite for convenience using Mis
	_, err = cache.Get("key")

	// A nice pattern to use to split the error handling from the logic of hit or not might be the following
	_, err = cache.Get("key")
	miss, err := lcache.Miss(err) // false, nil
	if err != nil {
		panic(err) // something went wrong with the sqlite database
	}
	if miss {
		fmt.Println("key miss")
		// key miss
	}

}