package cove

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Op func(*Cache) error

func dbPragma(pragma string) Op {
	return func(c *Cache) error {
		return exec(c.db,
			fmt.Sprintf(`pragma %s;`, pragma),
		)
	}
}

// WithLogger sets the logger for the cache
func WithLogger(log *slog.Logger) Op {
	return func(c *Cache) error {
		if log == nil {
			log = slog.New(discardLogger{})
		}
		c.log = log
		return nil
	}
}

// WithVacuum sets the vacuum function to be called in a go routine
func WithVacuum(vacuum func(cache *Cache)) Op {
	return func(cache *Cache) error {
		cache.vacuum = vacuum
		return nil
	}
}

// WithTTL sets the default TTL for the cache
func WithTTL(defaultTTL time.Duration) Op {
	return func(c *Cache) error {
		if defaultTTL > time.Duration(0) {
			c.ttl = defaultTTL
		}
		return nil
	}
}

// WithEvictCallback sets the callback function to be called when a key is evicted
func WithEvictCallback(cb func(key string, val []byte)) Op {
	return func(c *Cache) error {
		c.onEvict = cb
		return nil
	}
}

// DBRemoveOnClose is a helper function to remove the database files on close
func DBRemoveOnClose() Op {
	return func(cache *Cache) error {
		*cache.removeOnClose = true
		return nil
	}
}

func dbDefault() Op {
	return func(c *Cache) error {
		return errors.Join(
			dbPragma("journal_mode = WAL")(c),
			dbPragma("synchronous = normal")(c),
			dbPragma(`auto_vacuum = incremental`)(c),
			dbPragma(`incremental_vacuum`)(c),
		)
	}
}

// DBSyncOff is a helper function to set
//
//	synchronous = off
//
// this is useful for write performance but effects read performance and durability
func DBSyncOff() Op {
	return func(c *Cache) error {
		return dbPragma("synchronous = off")(c)
	}
}

// DBPragma is a helper function to set a pragma on the database
//
// see https://www.sqlite.org/pragma.html
//
// example:
//
//	DBPragma("journal_size_limit = 6144000")
func DBPragma(s string) Op {
	return func(c *Cache) error {
		return dbPragma(s)(c)
	}
}

type Cache struct {
	//shards
	uri string

	ttl time.Duration

	onEvict func(key string, val []byte)

	namespace  string
	namespaces map[string]*Cache

	mu    *sync.Mutex
	muKey *keyedMutex

	db            *sql.DB
	closeOnce     *sync.Once
	closed        chan struct{}
	removeOnClose *bool
	log           *slog.Logger
	vacuum        func(cache *Cache)
}

func New(uri string, op ...Op) (*Cache, error) {
	db, err := sql.Open("sqlite3", uri)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not ping db, %w", err)
	}

	c := &Cache{
		uri: uri,
		ttl: NO_TTL,

		onEvict: nil,

		vacuum: Vacuum(5*time.Minute, 1_000), // default vacuum

		namespace:  NS_DEFAULT,
		namespaces: make(map[string]*Cache),
		mu:         &sync.Mutex{},
		muKey:      keyedMu(),
		db:         db,
		log:        slog.New(discardLogger{}),

		closeOnce:     &sync.Once{},
		closed:        make(chan struct{}),
		removeOnClose: new(bool),
	}
	*c.removeOnClose = false

	c.namespaces[c.namespace] = c

	ops := append([]Op{dbDefault()}, op...)

	c.log.Debug("[cove] applying options")
	for _, op := range ops {
		err := op(c)
		if err != nil {
			return nil, fmt.Errorf("could not exec op, %w", err)
		}
	}

	c.log.Debug("[cove] creating schema")
	err = schema(c)
	if err != nil {
		return nil, fmt.Errorf("failed in creating schema, %w", err)
	}

	c.log.Debug("[cove] optimizing database")
	err = optimize(c)
	if err != nil {
		return nil, fmt.Errorf("failed in optimizing, %w", err)
	}

	if c.vacuum != nil {
		c.log.Debug("[cove] starting vacuum")
		go c.vacuum(c)
	}

	return c, nil
}

// NS creates a new namespace, if the namespace already exists it will return the existing namespace.
//
// onEvict must be set for every new namespace created using WithEvictCallback.
// NS will create a new table in the database for the namespace in order to isolate it, and the indexes.
func (c *Cache) NS(ns string, ops ...Op) (*Cache, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, found := c.namespaces[ns]; found {
		return c.namespaces[ns], nil
	}

	nc := &Cache{
		uri: c.uri,
		ttl: c.ttl,

		onEvict: nil,
		vacuum:  c.vacuum,

		namespace:  ns,
		namespaces: c.namespaces,

		mu:    c.mu,
		muKey: keyedMu(),

		db: c.db,

		closeOnce:     c.closeOnce,
		closed:        c.closed,
		removeOnClose: c.removeOnClose,
		log:           c.log.With("ns", ns),
	}

	nc.namespaces[nc.namespace] = nc

	nc.log.Debug("[cove] creating schema")
	err := schema(nc)
	if err != nil {
		return nil, fmt.Errorf("could not create schema for namespace, %w", err)
	}

	nc.log.Debug("[cove] applying options")
	for _, op := range ops {
		err = op(c)
		if err != nil {
			return nil, fmt.Errorf("could not exec op, %w", err)
		}
	}

	return nc, nil
}

func (c *Cache) tbl() string {
	return "_cache_" + c.namespace
}

func optimize(c *Cache) error {
	return exec(c.db, `pragma vacuum;`, `pragma optimize;`)
}

func schema(c *Cache) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    			key TEXT primary key, 
    			value BLOB,
    			create_at INTEGER DEFAULT (strftime('%%s', 'now')),
    			expire_at INTEGER,
    			ttl INTEGER                
		 );`, c.tbl())

	return exec(c.db, q)
}

// Close closes the cache and all its namespaces
func (c *Cache) Close() error {
	defer func() {
		if *c.removeOnClose {
			_ = c.removeStore()
		}
	}()

	defer func() {
		c.closeOnce.Do(func() {
			close(c.closed)
		})
	}()

	return c.db.Close()
}

func (c *Cache) removeStore() error {

	select {
	case <-c.closed:
	default:
		return fmt.Errorf("db is not closed")
	}

	schema, uri, found := strings.Cut(c.uri, ":")
	if !found {
		return fmt.Errorf("could not find file in uri")
	}
	if schema != "file" {
		return fmt.Errorf("not a file uri")
	}

	file, query, _ := strings.Cut(uri, "?")

	c.log.Info("[cove] remove store", "db", file, "shm", fmt.Sprintf("%s-shm", file), "wal", fmt.Sprintf("%s-wal", file))

	err := errors.Join(
		os.Remove(file),
		os.Remove(fmt.Sprintf("%s-shm", file)),
		os.Remove(fmt.Sprintf("%s-wal", file)),
	)
	if strings.Contains(query, "tmp=true") {
		c.log.Info("[cove] remove store dir", "dir", filepath.Dir(file))
		err = errors.Join(os.Remove(filepath.Dir(file)), err)
	}

	return err
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) ([]byte, error) {
	return get(c.db, key, c.tbl())
}

// GetOr retrieves a value from the cache, if the key does not exist it will call the setter function and set the result.
//
// If multiple goroutines call GetOr with the same key, only one will call the fetch function
// the others will wait for the first to finish and retrieve the cached value from the first call.
// It is useful paradigm to lessen a thundering herd problem.
// This is done by locking on the provided key in the application layer, not the database layer.
// meaning, this might work poorly if multiple applications are using the same sqlite cache files.
func (c *Cache) GetOr(key string, fetch func(k string) ([]byte, error)) ([]byte, error) {
	var err error
	var value []byte

	value, err = get(c.db, key, c.tbl())

	if err == nil {
		return value, nil
	}

	if !errors.Is(err, NotFound) {
		return nil, fmt.Errorf("could not get key %s, err; %w", key, err)
	}

	if errors.Is(err, NotFound) {
		c.muKey.Lock(key)
		defer c.muKey.Unlock(key)

		value, err = get(c.db, key, c.tbl()) // if someone else has set the key
		if err == nil {
			return value, nil
		}

		c.log.Debug("[cove] cache miss fetching value, using fetcher", "key", key)

		value, err = fetch(key)
		if err != nil {
			return nil, fmt.Errorf("could not set value, err; %w", err)
		}
		err = setTTL(c.db, key, value, c.ttl, c.tbl())
		if err != nil {
			return nil, fmt.Errorf("could not set ttl, err; %w", err)
		}
		return value, nil

	}
	return value, nil
}

// Set sets a value in the cache, with default ttl
func (c *Cache) Set(key string, value []byte) error {
	return c.SetTTL(key, value, c.ttl)
}

// SetTTL sets a value in the cache with a custom ttl
func (c *Cache) SetTTL(key string, value []byte, ttl time.Duration) error {
	return setTTL(c.db, key, value, ttl, c.tbl())
}

func (c *Cache) tx(eval func(tx *sql.Tx) error) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("could not begin tx, err; %w", err)
	}
	err = eval(tx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("could not eval tx, err; %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit tx, err; %w", err)
	}
	return nil

}

// BatchSet sets a batch of key/value pairs in the cache
//
// the BatchSet will take place in one transaction, but split up into sub-batches of MAX_PARAMS/3 size, ie 999/3 = 333,
// in order to have the BatchSet be atomic. If one key fails to set, the whole batch will fail.
// Prefer batches less then MAX_PARAMS
func (c *Cache) BatchSet(rows []KV[[]byte]) error {
	size := MAX_PARAMS / 3

	if len(rows) <= size {
		return batchSet(c.db, rows, c.ttl, c.tbl())
	}

	err := c.tx(func(tx *sql.Tx) error {
		for i := 0; i < len(rows); i += size {
			end := i + size
			if end > len(rows) {
				end = len(rows)
			}
			chunk := rows[i:end]
			err := batchSet(tx, chunk, c.ttl, c.tbl())
			if err != nil {
				return fmt.Errorf("could not batch set, err; %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("could not set full batch, err; %w", err)
	}

	return nil
}

// BatchGet retrieves a batch of keys from the cache
//
// the BatchGet will take place in one transaction, but split up into sub-batches of MAX_PARAMS size, ie 999,
// in order to have the BatchGet be atomic. If one key fails to fetched, the whole batch will fail.
// Prefer batches less then MAX_PARAMS
func (c *Cache) BatchGet(keys []string) ([]KV[[]byte], error) {

	size := MAX_PARAMS

	if len(keys) <= size {
		return batchGet(c.db, keys, c.tbl())
	}

	var res []KV[[]byte]

	err := c.tx(func(tx *sql.Tx) error {
		for i := 0; i < len(keys); i += size {
			end := i + size
			if end > len(keys) {
				end = len(keys)
			}
			chunk := keys[i:end]
			kvs, err := batchGet(tx, chunk, c.tbl())
			if err != nil {
				return fmt.Errorf("could not batch get, err; %w", err)
			}
			res = append(res, kvs...)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not get full batch, err; %w", err)
	}

	return res, nil
}

// BatchEvict evicts a batch of keys from the cache
//
// if onEvict is set, it will be called for each key
// the eviction will take place in one transaction, but split up into bacthes of MAX_PARAMS, ie 999,
// in order to have the eviction be atomic. If one key fails to evict, the whole batch will fail.
// Prefer batches less then MAX_PARAMS
func (c *Cache) BatchEvict(keys []string) (evicted []KV[[]byte], err error) {

	defer func() {
		if c.onEvict != nil {
			for _, kv := range evicted {
				key := kv.K
				val := kv.V
				go c.onEvict(key, val)
			}
		}
	}()

	size := MAX_PARAMS
	if len(keys) <= size {
		evicted, err = batchEvict(c.db, keys, c.tbl())
		return evicted, err
	}

	err = c.tx(func(tx *sql.Tx) error {
		for i := 0; i < len(keys); i += size {
			end := i + size
			if end > len(keys) {
				end = len(keys)
			}
			chunk := keys[i:end]
			kvs, err := batchEvict(tx, chunk, c.tbl())
			if err != nil {
				return fmt.Errorf("could not batch evict, err; %w", err)
			}
			evicted = append(evicted, kvs...)
		}
		return nil
	})

	return evicted, err
}

// Evict evicts a key from the cache
// if onEvict is set, it will be called for key
func (c *Cache) Evict(key string) (kv KV[[]byte], err error) {
	kv, err = evict(c.db, key, c.tbl())
	if err == nil && c.onEvict != nil {
		go c.onEvict(kv.K, kv.V)
	}
	return kv, err
}

// EvictAll evicts all keys in the cache
// onEvict will not be called
func (c *Cache) EvictAll() (len int, err error) {
	return evictAll(c.db, c.tbl())
}

// Range returns all key value pairs in the range [from, to]
func (c *Cache) Range(from string, to string) (kv []KV[[]byte], err error) {
	return getRange(c.db, from, to, c.tbl())
}

// Keys returns all keys in the range [from, to]
func (c *Cache) Keys(from string, to string) (keys []string, err error) {
	return getKeys(c.db, from, to, c.tbl())
}

// Values returns all values in the range [from, to]
func (c *Cache) Values(from string, to string) (values [][]byte, err error) {
	return getValues(c.db, from, to, c.tbl())
}

// ItrRange returns an iterator for the range of keys [from, to]
//
//	WARNING
//	Since iterators don't really have any way of communication errors
//	the Con is that errors are dropped when using iterators.
//	the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)
func (c *Cache) ItrRange(from string, to string) iter.Seq2[string, []byte] {
	return iterKV(c.db, from, to, c.tbl(), c.log)
}

// ItrKeys returns an iterator for the range of keys [from, to]
//
//	WARNING
//	Since iterators don't really have any way of communication errors
//	the Con is that errors are dropped when using iterators.
//	the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)
func (c *Cache) ItrKeys(from string, to string) iter.Seq[string] {
	return iterKeys(c.db, from, to, c.tbl(), c.log)
}

// ItrValues returns an iterator for the range of values [from, to]
//
//	WARNING
//	Since iterators don't really have any way of communication errors
//	the Con is that errors are dropped when using iterators.
//	the Pro is that it is very easy to use, and scan row by row (ie. no need to load all rows into memory)
func (c *Cache) ItrValues(from string, to string) iter.Seq[[]byte] {
	return iterValues(c.db, from, to, c.tbl(), c.log)
}

func (c *Cache) Vacuum(max int) (n int, err error) {

	c.log.Debug("[cove] vacuuming namespace", "max_eviction", max)
	if c.onEvict == nil { // Dont do expensive vacuum if no onEvict is set
		return vacuumNoResult(c.db, max, c.tbl())
	}

	kvs, err := vacuum(c.db, max, c.tbl())
	if err != nil {
		return 0, fmt.Errorf("could not vacuum, err; %w", err)
	}

	if c.onEvict != nil {
		c.log.Debug("[cove] calling onEvict callback", "evicted", len(kvs))
		for _, kv := range kvs {
			k := kv.K
			v := kv.V
			go c.onEvict(k, v)
		}
	}
	return len(kvs), nil
}
