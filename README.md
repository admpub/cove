# Cove

`cove` is a caching library for Go that utilizes SQLite as the storage backend. It provides a simple and efficient way to cache key-value pairs with support for TTL (Time-To-Live), namespaces, batch operations, range scans and eviction callbacks.


## TL;DR

Simple caching library for Go backed by SQLite

```bash 
go get github.com/modfin/cove
```

```go
package main

import (
	"fmt"
	"github.com/modfin/cove"
	"strings"
	"time"
)

func main() {
	cache, err := cove.New(
		cove.URITemp(),
	)
	if err != nil {
		panic(err)
	}
	defer cache.Close()

	ns, err := cache.NS("strings")
	if err != nil {
		panic(err)
	}
	stringCache := cove.Of[string](ns)

	stringCache.Set("key", "the string")

	str, err := stringCache.Get("key")
	found, err := cove.Hit(err)
	if err != nil {
		panic(err)
	}
	if found {
		fmt.Println(str) // Output: the string
	}

}

```


## Use case
cove is meant to be embedded into your application, and not as a standalone service. It is a simple key-value store that is meant to be used for caching data that is expensive to compute or retrieve. 

cove can also be used as simple key-value store

using SQLite as a storage backend comes with some benefits such as
- Transactions and ACID
- Key-Value pairs up to a size of 1GB each
- Offloading to disk allowing for very larger caches
- Easy to use and setup
- No need for a separate service

## Installation

To install `cove`, use `go get`:

```sh
go get github.com/modfin/cove
```

## Usage

### Creating a Cache

To create a cache, use the `New` function. You can specify various options such as TTL, vacuum interval, and eviction callbacks.

```go
package main

import (
    "github.com/modfin/cove"
    "time"
)

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
        cove.WithTTL(time.Minute*10),
    )
    if err != nil {
        panic(err)
    }
    defer cache.Close()
}
```

### Setting and Getting Values

You can set and get values from the cache using the `Set` and `Get` methods.

```go
package main

import (
    "fmt"
    "github.com/modfin/cove"
    "time"
)

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
        cove.WithTTL(time.Minute*10),
    )
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    // Set a value
    err = cache.Set("key", []byte("value0"))
    if err != nil {
        panic(err)
    }

    // Get the value
    value, err := cache.Get("key")
    if err != nil {
        panic(err)
    }

    fmt.Println(string(value)) // Output: value0
}
```

### Using Namespaces

Namespaces allow you to isolate different sets of keys within the same cache.

```go
package main

import (
    "github.com/modfin/cove"
    "time"
	"fmt"
)

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
        cove.WithTTL(time.Minute*10),
    )
    if err != nil {
        panic(err)
    }
    defer cache.Close()


	err = cache.Set("key", []byte("value0"))
	if err != nil {
		panic(err)
	}
	
    ns, err := cache.NS("namespace1")
    if err != nil {
        panic(err)
    }

    err = ns.Set("key", []byte("value1"))
    if err != nil {
        panic(err)
    }

    value, err := ns.Get("key")
    if err != nil {
        panic(err)
    }

    fmt.Println(string(value)) // Output: value1
}
```

### Eviction Callbacks

You can set a callback function to be called when a key is evicted from the cache.

```go
package main

import (
    "fmt"
    "github.com/modfin/cove"
)

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
        cove.WithEvictCallback(func(key string, val []byte) {
            fmt.Printf("evicted %s: %s\n", key, string(val))
        }),
    )
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    err = cache.Set("key", []byte("evict me"))
    if err != nil {
        panic(err)
    }

    _, err = cache.Evict("key")
    if err != nil {
        panic(err)
    }
    // Output: evicted key: evict me
}
```

### Using Iterators

Iterators allow you to scan through keys and values without loading all rows into memory.

> **WARNING** \
> Since iterators don't really have any way of communication errors \
> the Con is that errors are dropped when using iterators. \
> the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)

```go
package main

import (
    "fmt"
    "github.com/modfin/cove"
    "time"
)

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
        cove.WithTTL(time.Minute*10),
    )
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    for i := 0; i < 100; i++ {
        err = cache.Set(fmt.Sprintf("key%d", i), []byte(fmt.Sprintf("value%d", i)))
        if err != nil {
            panic(err)
        }
    }

    // KV iterator
    for k, v := range cache.ItrRange("key97", cove.RANGE_MAX) {
        fmt.Println(k, string(v))
    }

    // Key iterator
    for key := range cache.ItrKeys(cove.RANGE_MIN, "key1") {
        fmt.Println(key)
    }

    // Value iterator
    for value := range cache.ItrValues(cove.RANGE_MIN, "key1") {
        fmt.Println(string(value))
    }
}
```

### Handling `NotFound` errors

If a key is not found in the cache, the `Get` method will return an `NotFound` error.

You can handle not found errors using the `Hit` and `Miss` helper functions.

```go
package main

import (
    "errors"
    "fmt"
    "github.com/modfin/cove"
    "time"
)

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
        cove.WithTTL(time.Minute*10),
    )
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    _, err = cache.Get("key")
    fmt.Println("err == cove.NotFound:", err == cove.NotFound)
    fmt.Println("errors.Is(err, cove.NotFound):", errors.Is(err, cove.NotFound))

    _, err = cache.Get("key")
    hit, err := cove.Hit(err)
    if err != nil {
        panic(err)
    }
    if !hit {
        fmt.Println("key miss")
    }

    _, err = cache.Get("key")
    miss, err := cove.Miss(err)
    if err != nil {
        panic(err)
    }
    if miss {
        fmt.Println("key miss")
    }
}
```


### Typed Cache

The `TypedCache` in `cove` provides a way to work with strongly-typed values in the cache, using Go generics. This allows you to avoid manual serialization and deserialization of values, making the code cleaner and less error-prone.

#### Creating a Typed Cache

To create a typed cache, use the `Of` function, passing the existing cache/namespace instance:

```go
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
	// Create a base cache
	cache, err := cove.New(
		cove.URITemp(),
		cove.DBRemoveOnClose(),
		cove.WithTTL(time.Minute*10),
	)
	assertNoErr(err)
	defer cache.Close()

	personNamespace, err := cache.NS("person")
	assertNoErr(err)
	
	// Create a typed cache for Person struct
	typedCache := cove.Of[Person](personNamespace)

	// Set a value in the typed cache
	err = typedCache.Set("alice", Person{Name: "Alice", Age: 30})
	assertNoErr(err)

	// Get a value from the typed cache
	alice, err := typedCache.Get("alice")
	assertNoErr(err)
	fmt.Printf("%+v\n", alice) // Output: {Name:Alice Age:30}
}
```

#### Methods

- **Set**: Sets a value in the cache with the default TTL.
- **Get**: Retrieves a value from the cache.
- **SetTTL**: Sets a value in the cache with a custom TTL.
- **GetOr**: Retrieves a value from the cache or calls a fetch function if the key does not exist.
- **BatchSet**: Sets a batch of key/value pairs in the cache.
- **BatchGet**: Retrieves a batch of keys from the cache.
- **BatchEvict**: Evicts a batch of keys from the cache.
- **Evict**: Evicts a key from the cache.
- **EvictAll**: Evicts all keys in the cache.
- **Range**: Returns all key-value pairs in a specified range.
- **Keys**: Returns all keys in a specified range.
- **Values**: Returns all values in a specified range.
- **ItrRange**: Returns an iterator for key-value pairs in a specified range.
- **ItrKeys**: Returns an iterator for keys in a specified range.
- **ItrValues**: Returns an iterator for values in a specified range.
- **Raw**: Returns the underlying untyped cache.

The `TypedCache` uses `encoding/gob` for serialization and deserialization of values, ensuring type safety and ease of use.




## License

This project is licensed under the MIT License - see the `LICENSE.md` file for details.
