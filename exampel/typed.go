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

type Person struct {
	Name string
	Age  int
}

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

	typed := lcache.Of[Person](cache)
	assertNoErr(err)

	// set a key value pair in the cache
	err = typed.Set("alice", Person{Name: "Alice", Age: 30})
	assertNoErr(err)

	err = typed.Set("bob", Person{Name: "Bob", Age: 40})
	assertNoErr(err)

	err = typed.Set("charlie", Person{Name: "Bob", Age: 40})
	assertNoErr(err)

	// get the value from the cache
	alice, err := typed.Get("alice")
	assertNoErr(err)

	fmt.Printf("%+v\n", alice)
	// {Name:Alice Age:30}

	zero, err := typed.Get("does-not-exist")
	fmt.Println("zero:", fmt.Sprintf("%+v", zero))
	// zero: {Name: Age:0}

	fmt.Println("err == lcache.NotFound:", err == lcache.NotFound)
	// err == lcache.NotFound: true

}
