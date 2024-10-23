package lcache

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/modfin/lcache/internal/lock"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// https://phiresky.github.io/blog/2020/sqlite-performance-tuning/

const NS_DEFAULT = "default"

var NotFound = errors.New("not found")

type Op func(*Cache) error

func exec(db *sql.DB, qs ...string) error {
	for _, q := range qs {
		_, err := db.Exec(q)
		if err != nil {
			return fmt.Errorf("could not exec, %s, err, %w", q, err)
		}
	}
	return nil
}

func dbPragma(pragma string) Op {
	return func(c *Cache) error {
		return exec(c.db,
			fmt.Sprintf(`pragma %s;`, pragma),
		)
	}
}

func WithVacuum(interval time.Duration) Op {
	return func(cache *Cache) error {
		cache.vacuumTimer = interval
		return nil
	}
}
func WithTTL(defaultTTL time.Duration) Op {
	return func(c *Cache) error {
		if defaultTTL > time.Duration(0) {
			c.ttl = defaultTTL
		}
		return nil
	}
}

func WithEvictCallback(cb func(key string, ns string)) Op {
	return func(c *Cache) error {
		c.onEvict = cb
		return nil
	}
}

func DBDefault() Op {
	return func(c *Cache) error {
		return errors.Join(
			dbPragma("journal_mode = WAL")(c),
			dbPragma("synchronous = normal")(c),
		)
	}
}

func DBSyncOff() Op {
	return func(c *Cache) error {
		return dbPragma("synchronous = off")(c)
	}
}

func DBFullOptimize() Op {
	return func(c *Cache) error {
		return errors.Join(
			dbPragma(`journal_mode = WAL`)(c),
			dbPragma(`synchronous = normal`)(c),
			dbPragma(`journal_size_limit = 6144000`)(c),
			dbPragma(`temp_store = memory`)(c),
			dbPragma(`mmap_size = 30000000000`)(c),
			dbPragma(`page_size = 32768`)(c),
			dbPragma(`auto_vacuum = incremental`)(c),
			dbPragma(`incremental_vacuum`)(c),
		)
	}
}

type Cache struct {
	//shards
	uri string

	ttl         time.Duration
	vacuumTimer time.Duration

	onEvict func(key string, ns string)

	ns  string
	ref map[string]*Cache

	mu    *sync.Mutex
	muKey *lock.KeyedMutex

	db     *sql.DB
	closed bool
}

func (c *Cache) NS(ns string, ops ...Op) (*Cache, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, found := c.ref[ns]; found {
		return c.ref[ns], nil
	}

	nc := &Cache{
		uri:         c.uri,
		ttl:         c.ttl,
		vacuumTimer: c.vacuumTimer,
		//onEvict:     nil,
		ns:     ns,
		ref:    c.ref,
		mu:     c.mu,
		muKey:  lock.Keyed(),
		db:     c.db,
		closed: c.closed,
	}

	nc.ref[nc.ns] = nc

	err := schema(nc)
	if err != nil {
		return nil, fmt.Errorf("could not create schema for namespace, %w", err)
	}

	for _, op := range ops {
		err = op(c)
		if err != nil {
			return nil, fmt.Errorf("could not exec op, %w", err)
		}
	}

	return nc, nil
}

func TempUri() string {
	d := fmt.Sprintf("%d-lcache", time.Now().Unix())
	uri := filepath.Join(os.TempDir(), d, "lcache.db")
	os.MkdirAll(filepath.Dir(uri), 0755)
	return fmt.Sprintf("file:%s?tmp=true", uri)
}

func (c *Cache) tbl() string {
	return "_cache_" + c.ns
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

func optimize(c *Cache) error {
	return exec(c.db, `pragma vacuum;`, `pragma optimize;`)
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
		uri:         uri,
		db:          db,
		ns:          NS_DEFAULT,
		ref:         make(map[string]*Cache),
		ttl:         72 * time.Hour,
		vacuumTimer: time.Minute,
		mu:          &sync.Mutex{},
		muKey:       lock.Keyed(),
	}
	c.ref[c.ns] = c

	ops := append([]Op{DBDefault()}, op...)
	for _, op := range ops {
		err := op(c)
		if err != nil {
			return nil, fmt.Errorf("could not exec op, %w", err)
		}
	}
	err = schema(c)
	if err != nil {
		return nil, fmt.Errorf("failed in creating schema, %w", err)
	}

	err = optimize(c)
	if err != nil {
		return nil, fmt.Errorf("failed in optimizing, %w", err)
	}

	return c, nil
}

func (c *Cache) Close() error {
	defer func() {
		c.closed = true
	}()
	return c.db.Close()
}

func (c *Cache) DeleteStore() error {
	if !c.closed {
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

	err := errors.Join(
		os.Remove(file),
		os.Remove(fmt.Sprintf("%s-shm", file)),
		os.Remove(fmt.Sprintf("%s-wal", file)),
	)

	if strings.Contains(query, "tmp=true") {
		err = errors.Join(os.Remove(filepath.Dir(file)), err)
	}

	return err
}

func (c *Cache) cleaner() {
	timer := time.Tick(c.vacuumTimer)
	for {
		select {
		case <-timer:
		}

		q := fmt.Sprintf(`DELETE FROM %s WHERE expire_at < strftime('%%s', 'now') RETURNING key;`, c.tbl())

		rows, err := c.db.Query(q)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			_, _ = fmt.Fprintf(os.Stderr, "could not delete expired keys, %v", err)
			continue
		}

		if c.onEvict != nil {
			for rows.Next() {
				var key string
				err := rows.Scan(&key)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "could not scan key, %v", err)
					continue
				}
				go c.onEvict(key, c.ns)
			}
		}
	}
}

func (c *Cache) tx(eval func(tx *sql.Tx) error) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("could not begin transaction, %w", err)
	}
	err = eval(tx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("rolling back due to err; %w", err)
	}
	err = tx.Commit()
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}

func (c *Cache) Get(key string) ([]byte, error) {
	return get(c.db, key, c.tbl)
}

func (c *Cache) GetOrSet(key string, setter func(k string) ([]byte, error)) ([]byte, error) {
	var err error
	var value []byte

	value, err = get(c.db, key, c.tbl)

	if err == nil {
		return value, nil
	}

	if !errors.Is(err, NotFound) {
		return nil, fmt.Errorf("could not get key %s, err; %w", key, err)
	}

	if errors.Is(err, NotFound) {
		c.muKey.Lock(key)
		defer c.muKey.Unlock(key)

		value, err = get(c.db, key, c.tbl) // if someone else has set the key
		if err == nil {
			return value, nil
		}

		value, err = setter(key)
		if err != nil {
			return nil, fmt.Errorf("could not set value, err; %w", err)
		}
		err = setTTL(c.db, key, value, c.ttl, c.tbl)
		if err != nil {
			return nil, fmt.Errorf("could not set ttl, err; %w", err)
		}
		return value, nil

	}
	return value, nil
}

func (c *Cache) Set(key string, value []byte) error {
	return c.SetTTL(key, value, c.ttl)
}

func (c *Cache) SetTTL(key string, value []byte, ttl time.Duration) error {

	return setTTL(c.db, key, value, ttl, c.tbl)
}

func (c *Cache) Evict(key string) (prev []byte, found bool, err error) {
	prev, found, err = evict(c.db, key, c.tbl)
	if err == nil && found && c.onEvict != nil {
		go c.onEvict(key, c.ns)
	}
	return prev, found, err
}
func (c *Cache) EvictAll() (len int, err error) {
	return evictAll(c.db, c.tbl)
}

type KV struct {
	Key   string
	Value []byte
}

func (c *Cache) Range(from string, to string) (kv []KV, err error) {
	return getrange(c.db, from, to, c.tbl)
}

type query interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func getrange(db query, from string, to string, tbl func() string) ([]KV, error) {
	r, err := db.Query(fmt.Sprintf(`SELECT key, value FROM %s WHERE $1 <= key AND key <= $2`, tbl()), from, to)
	if err != nil {
		return nil, fmt.Errorf("could not query, %w", err)
	}
	defer r.Close()
	var kv []KV
	for r.Next() {
		var k, v string
		err := r.Scan(&k, &v)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kv = append(kv, KV{k, []byte(v)})

	}
	return kv, nil
}

func evictAll(r query, tbl func() string) (int, error) {
	res, err := r.Exec(fmt.Sprintf(`
		DELETE FROM %s 
	`, tbl()))
	if err != nil {
		return 0, fmt.Errorf("could not delete all in %s, err; %w", tbl(), err)
	}
	i, err := res.RowsAffected()
	return int(i), err
}
func evict(r query, key string, tbl func() string) ([]byte, bool, error) {
	var value []byte
	err := r.QueryRow(fmt.Sprintf(`
		DELETE FROM %s 
		WHERE key = $1
		RETURNING value
	`, tbl()), key).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return value, true, err
}

func get(r query, key string, tbl func() string) ([]byte, error) {

	do := func() ([]byte, error) {
		var value []byte
		err := r.QueryRow(fmt.Sprintf(`
		SELECT value FROM %s 
		WHERE key = $1 AND strftime('%%s', 'now') < expire_at
	`, tbl()), key).Scan(&value)
		return value, err
	}
	value, err := do()

	if errors.Is(err, sql.ErrNoRows) {
		return nil, NotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not get key %s, err; %w", key, err)
	}
	return value, nil
}

func setTTL(r query, key string, value []byte, ttl time.Duration, tbl func() string) error {

	do := func() error {
		_, err := r.Exec(fmt.Sprintf(`
		INSERT INTO %s (key, value, expire_at, ttl) 
		VALUES ($1, $2, strftime('%%s', 'now')+$3, $3)
		ON CONFLICT (key)
		    DO UPDATE 
		    SET 
			  value = excluded.value, 
			  create_at =  excluded.create_at, 
			  expire_at = excluded.expire_at, 
			  ttl =  excluded.ttl
	`, tbl()), key, string(value), int(ttl.Seconds()))
		return err
	}

	err := do()

	if err != nil {
		return fmt.Errorf("could not exec, %w", err)
	}
	return nil
}
