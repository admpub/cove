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

type Person struct {
	Name string
	Age  int
}

func main() {

	// creates a sqlite cache in a temporary directory,
	//  once the cache is closed the database is removed
	//  a default TTL of 10 minutes is set
	cache, err := cove.New(cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithTTL(time.Minute*10),
	)
	assertNoErr(err)
	defer cache.Close()

	typed := cove.Of[Person](cache)
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

	fmt.Println("err == cove.NotFound:", err == cove.NotFound)
	// err == cove.NotFound: true

}
