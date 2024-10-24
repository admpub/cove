package main

import (
	"fmt"
	"github.com/modfin/cove"
	"sync"
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

	fetch := func(key string) ([]byte, error) {
		fmt.Println("fetching value for key:", key)

		nano := time.Now().UnixNano()
		time.Sleep(500 * time.Millisecond)
		val := fmt.Sprintf("async-value-for-%s, at: %d", key, nano)
		return []byte(val), nil
	}

	// GetOr will return the value if it exists, otherwise it will call the fetch function
	// if multiple goroutines call GetOr with the same key, only one will call the fetch function
	// the others will wait for the first to finish and retrieve the cached value from the first call.
	// It is useful to avoid thundering herd problem.
	// This is done by locking on the provided key in the application layer, not the database layer.
	// meaning, this might work poorly if multiple applications are using the same sqlite cache files.
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			val, err := cache.GetOr("MyKey", fetch)
			assertNoErr(err)
			fmt.Println(string(val))
			wg.Done()
		}()
	}

	wg.Wait()
	// fetching value for key: MyKey
	// async-value-for-MyKey, at: 1729775954803141713
	// async-value-for-MyKey, at: 1729775954803141713
	// async-value-for-MyKey, at: 1729775954803141713
	// async-value-for-MyKey, at: 1729775954803141713
	// ....
	// ....
	// ....

	// Seperate namespace can of course lock on the same key without interfering with each other
	cache2, err := cache.NS("cache2")

	wg = sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			val, err := cache.GetOr("MyOtherKey", fetch)
			assertNoErr(err)
			fmt.Println("[cache1]", string(val))
			wg.Done()
		}()

		go func() {
			val, err := cache2.GetOr("MyOtherKey", fetch)
			assertNoErr(err)
			fmt.Println("[cache2]", string(val))
			wg.Done()
		}()
	}
	wg.Wait()
	//fetching value for key: MyOtherKey                              // one fetch from cache1
	//fetching value for key: MyOtherKey                              //   and one from cache2
	//[cache1] async-value-for-MyOtherKey, at: 1729776686806490668
	//[cache2] async-value-for-MyOtherKey, at: 1729776686806426035
	//[cache2] async-value-for-MyOtherKey, at: 1729776686806426035
	//[cache1] async-value-for-MyOtherKey, at: 1729776686806490668
	//[cache1] async-value-for-MyOtherKey, at: 1729776686806490668
	//[cache2] async-value-for-MyOtherKey, at: 1729776686806426035
	//[cache1] async-value-for-MyOtherKey, at: 1729776686806490668
	// ....
	// ....
	// ....
}
