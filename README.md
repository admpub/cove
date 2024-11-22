# Cove

[![goreportcard.com](https://goreportcard.com/badge/github.com/admpub/cove)](https://goreportcard.com/report/github.com/admpub/cove)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/admpub/cove)](https://pkg.go.dev/github.com/admpub/cove)


`cove` is a caching library for Go that utilizes SQLite as the storage backend. It provides a TTL cache for key-value pairs with support for namespaces, batch operations, range scans and eviction callbacks.


## TL;DR

A TTL caching for Go backed by SQLite. (See examples for usage)

```bash 
go get github.com/admpub/cove
```

```go
package main

import (
	"fmt"
	"github.com/admpub/cove"
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
	// When using generic, use a separate namespace for each type
	stringCache := cove.Of[string](ns)

	stringCache.Set("key", "the string")

	str, err := stringCache.Get("key")
	hit, err := cove.Hit(err)
	if err != nil {
		panic(err)
	}
	if hit {
		fmt.Println(str) // Output: the string
	}

	str, err = stringCache.GetOr("async-key", func(key string) (string, error) {
		return "refreshed string", nil
	})
	hit, err = cove.Hit(err)
	if err != nil {
		panic(err)
	}
	if hit {
		fmt.Println(str) // Output: refreshed string
	}

}

```


## Use case
`cove` is meant to be embedded into your application, and not as a standalone service. 
It is a simple key-value store that is meant to be used for caching data that is expensive to compute or retrieve. 
`cove` can also be used as a key-value store

So why SQLite?

There are plenty of in memory and other caches build in go, \
eg https://github.com/avelino/awesome-go#cache, performance, concurrency, fast, LFU, LRU, ARC and so on. \
There are also a few key-value stores build in go that can be embedded (just like cove), \
eg https://github.com/dgraph-io/badger or https://github.com/boltdb/bolt and probably quite a few more, \
https://github.com/avelino/awesome-go#databases-implemented-in-go.

Well if these alternatives suits your use case, use them. 
The main benefit of using a cache/kv, from my perspective and the reason for building cove, is that a cache backed by sqlite should be decent 
and fast enough in most case. 
Its generically just a good solution while probably being outperformed by others in niche cases.
- you can have very large K/V pairs
- you can tune it for your use case
- it should perform decently
- you can cache hundreds of GB. SSD are fast these days.
- page caching and tuning will help you out.

While sqlite has come a long way since its inception and particular with it running in WAL mode, there are some limitations.
Eg only one writer is allowed at a time. So if you have a write heavy cache, you might want to consider another solution. 
With that said it should be fine for most with some tuning to increase write performance, eg `synchronous = off`.

## Installation

To install `cove`, use `go get`:

```sh
go get github.com/admpub/cove
```

### Considerations
Since `cove` uses SQLite as the storage backend, it is important realize that you project now will depend on cgo and that the SQLite library will be compiled into your project. This might not be a problem at all, but it could cause problems in some modern "magic" build tools used in CD/CI pipelines for go.

## Usage

### Tuning 

cove uses sqlite in WAL mode and writes the data to disk. While probably :memory: works for the most part, it does not have all the cool performance stuff that comes with sqlite in WAL mode on disk and probably will result in som SQL_BUSY errors.

In general the default tuning is the following
```sqlite
PRAGMA journal_mode = WAL;
PRAGMA synchronous = normal;
PRAGMA temp_store = memory;
PRAGMA auto_vacuum = incremental;
PRAGMA incremental_vacuum;
```

Have a look at https://www.sqlite.org/pragma.html for tuning your cache to your needs.

If you are write heavy, you might want to consider `synchronous = off` and dabble with some other settings, eg `wal_autocheckpoint`, to increase write performance. The tradeoff is that you might lose some read performance instead.

```go
    cache, err := cove.New(
		cove.URITemp(), 
		cove.DBSyncOff(), 
		// Yes, yes, this can be used to inject sql, but I trust you
		// to not let your users arbitrarily configure pragma on your 
		// sqlite instance.
		cove.DBPragma("wal_autocheckpoint = 1000"),
    )
```


### Creating a Cache

To create a cache, use the `New` function. You can specify various options such as TTL, vacuum interval, and eviction callbacks.


#### Config

`cove.URITemp()` Creates a temporary directory in `/tmp` or similar for the database. If combined with a `cove.DBRemoveOnClose()` and a gracefull shutdown, the database will be removed on close

if you want the cache to persist over restarts or such, you can use `cove.URIFromPath("/path/to/db/cache.db")` instead.

There are a few options that can be set when creating a cache, see `cove.With*` and `cove.DB*` functions for more information.

#### Example

```go
package main

import (
    "github.com/admpub/cove"
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
    "github.com/admpub/cove"
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
	hit, err := cove.Hit(err)
    if err != nil {
        panic(err)
    }

    fmt.Println("[Hit]:", hit, "[Value]:", string(value)) 
	// Output: "[Hit]: true [Value]: value0
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
    "github.com/admpub/cove"
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
    if err != nil { // A "real" error has occurred
        panic(err)
    }
    if !hit {
        fmt.Println("key miss")
    }

    _, err = cache.Get("key")
    miss, err := cove.Miss(err)
    if err != nil { // A "real" error has occurred
        panic(err)
    }
    if miss {
        fmt.Println("key miss")
    }
}
```





### Using Namespaces

Namespaces allow you to isolate different sets of keys within the same cache.

```go
package main

import (
    "github.com/admpub/cove"
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




	value, err := cache.Get("key")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(value)) // Output: value0
	
    value, err = ns.Get("key")
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
    "github.com/admpub/cove"
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
    "github.com/admpub/cove"
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


### Batch Operations

`cove` provides batch operations to efficiently handle multiple keys in a single operation. This includes `BatchSet`, `BatchGet`, and `BatchEvict`.

#### BatchSet

The `BatchSet` method allows you to set multiple key-value pairs in the cache in a single operation. This method ensures atomicity by splitting the batch into sub-batches if necessary.

```go
package main

import (
    "fmt"
    "github.com/admpub/cove"
)

func assertNoErr(err error) {
    if err != nil {
        panic(err)
    }
}

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
    )
    assertNoErr(err)
    defer cache.Close()

    err = cache.BatchSet([]cove.KV[[]byte]{
        {K: "key1", V: []byte("val1")},
        {K: "key2", V: []byte("val2")},
    })
    assertNoErr(err)
}
```

#### BatchGet

The `BatchGet` method allows you to retrieve multiple keys from the cache in a single operation. This method ensures atomicity by splitting the batch into sub-batches if necessary.

```go
package main

import (
    "fmt"
    "github.com/admpub/cove"
)

func assertNoErr(err error) {
    if err != nil {
        panic(err)
    }
}

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
    )
    assertNoErr(err)
    defer cache.Close()

    err = cache.BatchSet([]cove.KV[[]byte]{
        {K: "key1", V: []byte("val1")},
        {K: "key2", V: []byte("val2")},
    })
    assertNoErr(err)

    kvs, err := cache.BatchGet([]string{"key1", "key2", "key3"})
    assertNoErr(err)

    for _, kv := range kvs {
        fmt.Println(kv.K, "-", string(kv.V))
        // Output:
        // key1 - val1
        // key2 - val2
    }
}
```

#### BatchEvict

The `BatchEvict` method allows you to evict multiple keys from the cache in a single operation. This method ensures atomicity by splitting the batch into sub-batches if necessary.

```go
package main

import (
    "fmt"
    "github.com/admpub/cove"
)

func assertNoErr(err error) {
    if err != nil {
        panic(err)
    }
}

func main() {
    cache, err := cove.New(
        cove.URITemp(),
        cove.DBRemoveOnClose(),
    )
    assertNoErr(err)
    defer cache.Close()

    err = cache.BatchSet([]cove.KV[[]byte]{
        {K: "key1", V: []byte("val1")},
        {K: "key2", V: []byte("val2")},
    })
    assertNoErr(err)

    evicted, err := cache.BatchEvict([]string{"key1", "key2", "key3"})
    assertNoErr(err)

    for _, kv := range evicted {
        fmt.Println("Evicted,", kv.K, "-", string(kv.V))
        // Output:
        // Evicted, key1 - val1
        // Evicted, key2 - val2
    }
}
```

These batch operations help in efficiently managing multiple keys in the cache, ensuring atomicity and reducing the number of individual operations.



### Typed Cache

The `TypedCache` in `cove` provides a way to work with strongly-typed values in the cache, using Go generics. This allows you to avoid manual serialization and deserialization of values, making the code cleaner and less error-prone.

#### Creating a Typed Cache

A Typed Cache is simply to use golang generics to wrap the cache and provide type safety and ease of use.
The Typed Cache comes with the same fetchers and api as the untyped cache but adds a marshalling and unmarshalling layer on top of it. 
encoding/gob is used for serialization and deserialization of values.

To create a typed cache, use the `Of` function, passing the existing cache/namespace instance:

```go
package main

import (
	"fmt"
	"github.com/admpub/cove"
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




## Benchmarks

All models are wrong but some are useful. Not sure what category this falls under,
but here are some benchmarks `inserts/sec`, `reads/sec`, `write mb/sec` and `read mb/sec`.

In general Linux, 4 cores, a ssd and 32 gb of ram it seems to do some 
- 20-30k inserts/sec
- 200k reads/sec.
- writes 100-200 mb/sec
- reads 1000-2000 mb/sec. 

It seems fast enough...

```txt 
 
BenchmarkSetParallel/default-4                         28_256 insert/sec
BenchmarkSetParallel/sync-off-4                        36_523 insert/sec
BenchmarkSetParallel/sync-off+autocheckpoint-4         25_480 insert/sec

BenchmarkGetParallel/default-4                        192_668 reads/sec
BenchmarkGetParallel/sync-off-4                       238_714 reads/sec
BenchmarkGetParallel/sync-off+autocheckpoint-4        193_778 reads/sec

BenchmarkSetMemParallel/default+0.1mb-4                   273 write-mb/sec
BenchmarkSetMemParallel/default+1mb-4                     261 write-mb/sec
BenchmarkSetMemParallel/sync-off+0.1mb-4                  238 write-mb/sec
BenchmarkSetMemParallel/sync-off+1mb-4                    212 write-mb/sec

BenchmarkSetMem/default+0.1mb-4                           104 write-mb/sec
BenchmarkSetMem/default+1mb-4                             122 write-mb/sec
BenchmarkSetMem/sync-off+0.1mb-4                          219 write-mb/sec
BenchmarkSetMem/sync-off+1mb-4                            249 write-mb/sec

BenchmarkGetMemParallel/default+0.1mb-4                2_189 read-mb/sec
BenchmarkGetMemParallel/default+1mb-4                  1_566 read-mb/sec
BenchmarkGetMemParallel/sync-off+0.1mb-4               2_194 read-mb/sec
BenchmarkGetMemParallel/sync-off+1mb-4                 1_501 read-mb/sec

BenchmarkGetMem/default+0.1mb-4                          764 read-mb/sec
BenchmarkGetMem/default+1mb-4                            520 read-mb/sec
BenchmarkGetMem/sync-off+0.1mb-4                         719 read-mb/sec
BenchmarkGetMem/sync-off+1mb-4                           530 read-mb/sec

```


## TODO

- [ ] Add hooks or middleware for logging, metrics, eviction strategy etc.
- [ ] More testing

## License

This project is licensed under the MIT License - see the `LICENSE` file for details.
