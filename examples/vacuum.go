package main

import (
	"github.com/modfin/cove"
	"log/slog"
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
	//  a default TTL of 500ms
	//  a logger is set to default slog.Default()
	//  a vacuum is set to run every 100ms, and vacuum at most 1000 items at a time
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithTTL(time.Millisecond*500),

		cove.WithLogger(slog.Default()),

		// vacuum every 100ms, and vacuum at most 1000 items at a time
		// This might be important to calibrate from your use case, as it might cause a lot of writes to the database
		// along with on-evict calls for expired items.
		cove.WithVacuum(cove.Vacuum(100*time.Millisecond, 1_000)),

		// on-evict callback prints the key that is evicted
		cove.WithEvictCallback(func(key string, value []byte) {
			slog.Default().Info("evicting", "key", key)
		}),
	)
	assertNoErr(err)
	defer cache.Close()

	// set a key value pair in the cache
	err = cache.Set("key", []byte("value0"))
	assertNoErr(err)

	// set a key value pair in the cache
	err = cache.Set("key1", []byte("value0"))
	assertNoErr(err)

	// set a key value pair in the cache
	err = cache.Set("key2", []byte("value0"))
	assertNoErr(err)

	time.Sleep(time.Second)
	//2024/10/24 17:05:16 INFO [cove] vacuumed ns=default time=736.684µs n=3
	//2024/10/24 17:05:16 INFO evicting key=key
	//2024/10/24 17:05:16 INFO evicting key=key1
	//2024/10/24 17:05:16 INFO evicting key=key2

}
