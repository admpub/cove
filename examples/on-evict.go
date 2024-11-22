package main

import (
	"fmt"

	"github.com/admpub/cove"
	"github.com/admpub/cove/examples/helper"
)

func main() {

	// creates a sqlite cache named ./cove.db in the directory of the execution
	//  it adds a callback function for eviction notices
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithEvictCallback(
			func(key string, val []byte) {
				fmt.Printf("evicted %s: %s\n", key, string(val))
				// Maybe do som stuff, proactively refresh the cache
			}),
	)
	helper.AssertNoErr(err)
	cache.Close()

	_ = cache.Set("key", []byte("evict me"))
	_, _ = cache.Evict("key")
	// evicted key: evict me
	cache.Close()
}
