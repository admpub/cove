package cove

import (
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"strings"
	"time"
)

type query interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func exec(db query, qs ...string) error {
	for _, q := range qs {
		_, err := db.Exec(q)
		if err != nil {
			return fmt.Errorf("could not exec, %s, err, %w", q, err)
		}
	}
	return nil
}

func getRange(db query, from string, to string, tbl string) (kv []KV[[]byte], err error) {
	r, err := db.Query(fmt.Sprintf(`SELECT key, value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		return nil, fmt.Errorf("could not query in range, %w", err)
	}
	defer r.Close()
	for r.Next() {
		var k string
		var v []byte
		err := r.Scan(&k, &v)
		if err != nil {
			return nil, fmt.Errorf("could not scan in range, %w", err)
		}
		kv = append(kv, KV[[]byte]{K: k, V: v})
	}
	return kv, nil
}

func getKeys(db query, from string, to string, tbl string) (keys []string, err error) {
	r, err := db.Query(fmt.Sprintf(`SELECT key FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		return nil, fmt.Errorf("could not query in range, %w", err)
	}
	defer r.Close()
	for r.Next() {
		var k string
		err := r.Scan(&k)
		if err != nil {
			return nil, fmt.Errorf("could not scan in range, %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func getValues(db query, from string, to string, tbl string) (vals [][]byte, err error) {
	r, err := db.Query(fmt.Sprintf(`SELECT value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		return nil, fmt.Errorf("could not query in range, %w", err)
	}
	defer r.Close()
	for r.Next() {
		var v []byte
		err := r.Scan(&v)
		if err != nil {
			return nil, fmt.Errorf("could not scan in range, %w", err)
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func iterKV(db query, from string, to string, tbl string, log *slog.Logger) iter.Seq2[string, []byte] {
	r, err := db.Query(fmt.Sprintf(`SELECT key, value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		if log != nil {
			log.Error("[cove] iterKV, could not query in iter", "err", err)
			return func(yield func(string, []byte) bool) {}
		}
		_, _ = fmt.Fprintf(os.Stderr, "[cove] iterKV, could not query in iter, %v", err)
		return func(yield func(string, []byte) bool) {}
	}

	return func(yield func(string, []byte) bool) {
		defer r.Close()
		for r.Next() {
			var k string
			var v []byte
			err := r.Scan(&k, &v)
			if err != nil {
				if log != nil {
					log.Error("[cove] iterKV, could not scan in iter,", "err", err)
					return
				}
				_, _ = fmt.Fprintf(os.Stderr, "cove: iterKV, could not scan in iter, %v", err)
				return
			}
			if !yield(k, v) {
				return
			}
		}
	}
}

func iterKeys(db query, from string, to string, tbl string, log *slog.Logger) iter.Seq[string] {
	r, err := db.Query(fmt.Sprintf(`SELECT key FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		if log != nil {
			log.Error("[cove] could not query in iter,", "err", err)
			return func(yield func(string) bool) {}
		}
		_, _ = fmt.Fprintf(os.Stderr, "cove: could not query in iter, %v", err)
		return func(yield func(string) bool) {}
	}

	return func(yield func(string) bool) {
		defer r.Close()
		for r.Next() {
			var k string
			err := r.Scan(&k)
			if err != nil {

				if log != nil {
					log.Error("[cove] iterKeys, could not scan in iter,", "err", err)
					return
				}

				_, _ = fmt.Fprintf(os.Stderr, "cove: iterKeys, could not scan in iter, %v", err)
				return
			}
			if !yield(k) {
				return
			}
		}
	}
}

func iterValues(db query, from string, to string, tbl string, log *slog.Logger) iter.Seq[[]byte] {
	r, err := db.Query(fmt.Sprintf(`SELECT value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		if log != nil {
			log.Error("[cove] iterValues, could not query in iter,", "err", err)
			return func(yield func([]byte) bool) {}
		}
		_, _ = fmt.Fprintf(os.Stderr, "cove: iterValues, could not query in iter, %v", err)
		return func(yield func([]byte) bool) {}
	}

	return func(yield func([]byte) bool) {
		defer r.Close()
		for r.Next() {
			var v string
			err := r.Scan(&v)
			if err != nil {
				if log != nil {
					log.Error("[cove] iterValues, could not scan in iter,", "err", err)
					return
				}
				_, _ = fmt.Fprintf(os.Stderr, "cove: iterValues, could not scan in iter, %v", err)
				return
			}
			if !yield([]byte(v)) {
				return
			}
		}
	}
}

func evictAll(r query, tbl string) (int, error) {
	res, err := r.Exec(fmt.Sprintf(`
		DELETE FROM %s 
	`, tbl))
	if err != nil {
		return 0, fmt.Errorf("could not delete all in %s, err; %w", tbl, err)
	}
	i, err := res.RowsAffected()
	return int(i), err
}
func evict(r query, key string, tbl string) (KV[[]byte], error) {
	var value []byte
	err := r.QueryRow(fmt.Sprintf(`
		DELETE FROM %s 
		WHERE key = $1
		RETURNING value
	`, tbl), key).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		return KV[[]byte]{}, NotFound
	}
	if err != nil {
		return KV[[]byte]{}, fmt.Errorf("could not delete key %s, err; %w", key, err)
	}
	if value == nil {
		value = []byte{}
	}
	return KV[[]byte]{K: key, V: value}, err
}

func get(r query, key string, tbl string) ([]byte, error) {

	var value []byte
	err := r.QueryRow(fmt.Sprintf(`
		SELECT value FROM %s 
		WHERE key = $1 AND strftime('%%s', 'now') < expire_at
	`, tbl), key).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, NotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not get key %s, err; %w", key, err)
	}
	if value == nil {
		value = []byte{}
	}
	return value, nil
}

func setTTL(r query, key string, value []byte, ttl time.Duration, tbl string) error {
	if value == nil {
		value = []byte{}
	}

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
	`, tbl), key, value, int(ttl.Seconds()))

	if err != nil {
		return fmt.Errorf("could not exec, %w", err)
	}
	return nil
}

func batchGet(db query, keys []string, tbl string) ([]KV[[]byte], error) {
	var values []string
	var params []any
	for i, key := range keys {
		values = append(values, fmt.Sprintf("$%d", i+1))
		params = append(params, key)
	}

	q := fmt.Sprintf(`
		SELECT key, value FROM %s 
		WHERE key IN (%s) 
		AND strftime('%%s', 'now') < expire_at
	`, tbl, strings.Join(values, ", "))

	rows, err := db.Query(q, params...)
	if err != nil {
		return nil, fmt.Errorf("could not query, %w", err)
	}
	defer rows.Close()
	var kvs []KV[[]byte]
	for rows.Next() {
		var k string
		var v []byte
		err := rows.Scan(&k, &v)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kvs = append(kvs, KV[[]byte]{K: k, V: v})
	}
	return kvs, nil
}

func batchSet(db query, rows []KV[[]byte], ttl time.Duration, tbl string) error {

	var values []string
	var params []any

	for i, kv := range rows {
		values = append(values, fmt.Sprintf("($%d, $%d, strftime('%%s', 'now')+$%d, $%d)", i*3+1, i*3+2, i*3+3, i*3+3))
		params = append(params, kv.K, kv.V, int(ttl.Seconds()))
	}

	q := fmt.Sprintf(`
		INSERT INTO %s (key, value, expire_at, ttl) 
		VALUES %s
		ON CONFLICT (key)
		    DO UPDATE 
		    SET 
			  value = excluded.value, 
			  create_at =  excluded.create_at, 
			  expire_at = excluded.expire_at, 
			  ttl =  excluded.ttl
	`, tbl, strings.Join(values, ","))

	_, err := db.Exec(q, params...)
	return err
}

func batchEvict(r query, keys []string, tbl string) ([]KV[[]byte], error) {
	var values []string
	var params []any
	for i, key := range keys {
		values = append(values, fmt.Sprintf("$%d", i+1))
		params = append(params, key)
	}

	q := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE key IN (%s) 
		RETURNING key, value
	`, tbl, strings.Join(values, ", "))

	rows, err := r.Query(q, params...)
	if err != nil {
		return nil, fmt.Errorf("could not query, %w", err)
	}
	defer rows.Close()
	var kvs []KV[[]byte]
	for rows.Next() {
		var k string
		var v []byte
		err := rows.Scan(&k, &v)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kvs = append(kvs, KV[[]byte]{K: k, V: v})
	}

	return kvs, nil

}

func vacuumNoResult(r query, max int, tbl string) (int, error) {
	q := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE expire_at < strftime('%%s', 'now')
		  AND key in (SELECT key FROM %s WHERE expire_at < strftime('%%s', 'now') LIMIT $1)
	`, tbl, tbl)

	res, err := r.Exec(q, max)
	if err != nil {
		return 0, fmt.Errorf("could not query, %w", err)
	}
	i, err := res.RowsAffected()
	return int(i), err
}
func vacuum(r query, max int, tbl string) ([]KV[[]byte], error) {

	q := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE expire_at < strftime('%%s', 'now')
		  AND key in (SELECT key FROM %s WHERE expire_at < strftime('%%s', 'now') LIMIT $1)
		RETURNING key, value
	`, tbl, tbl)

	rows, err := r.Query(q, max)
	if err != nil {
		return nil, fmt.Errorf("could not query, %w", err)
	}
	defer rows.Close()
	var kvs []KV[[]byte]
	for rows.Next() {
		var k string
		var v []byte
		err := rows.Scan(&k, &v)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kvs = append(kvs, KV[[]byte]{K: k, V: v})
	}
	return kvs, nil
}
