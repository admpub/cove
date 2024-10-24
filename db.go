package lcache

import (
	"database/sql"
	"errors"
	"fmt"
	"iter"
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

func getRange(db query, from string, to string, tbl string) (kv []KV, err error) {
	r, err := db.Query(fmt.Sprintf(`SELECT key, value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		return nil, fmt.Errorf("could not query in range, %w", err)
	}
	defer r.Close()
	for r.Next() {
		var k, v string
		err := r.Scan(&k, &v)
		if err != nil {
			return nil, fmt.Errorf("could not scan in range, %w", err)
		}
		kv = append(kv, KV{K: k, V: []byte(v)})
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
		var v string
		err := r.Scan(&v)
		if err != nil {
			return nil, fmt.Errorf("could not scan in range, %w", err)
		}
		vals = append(vals, []byte(v))
	}
	return vals, nil
}

func iterKV(db query, from string, to string, tbl string) iter.Seq2[string, []byte] {
	r, err := db.Query(fmt.Sprintf(`SELECT key, value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lcache: could not query in iter, %v", err)
		return nil
	}

	return func(yield func(string, []byte) bool) {
		defer r.Close()
		for r.Next() {
			var k, v string
			err := r.Scan(&k, &v)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "lcache: could not scan in iter, %v", err)
				return
			}
			if !yield(k, []byte(v)) {
				return
			}
		}
	}
}

func iterKeys(db query, from string, to string, tbl string) iter.Seq[string] {
	r, err := db.Query(fmt.Sprintf(`SELECT key FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lcache: could not query in iter, %v", err)
		return nil
	}

	return func(yield func(string) bool) {
		defer r.Close()
		for r.Next() {
			var k string
			err := r.Scan(&k)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "lcache: could not scan in iter, %v", err)
				return
			}
			if !yield(k) {
				return
			}
		}
	}
}

func iterValues(db query, from string, to string, tbl string) iter.Seq[[]byte] {
	r, err := db.Query(fmt.Sprintf(`SELECT value FROM %s WHERE $1 <= key AND key <= $2`, tbl), from, to)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lcache: could not query in iter, %v", err)
		return nil
	}

	return func(yield func([]byte) bool) {
		defer r.Close()
		for r.Next() {
			var v string
			err := r.Scan(&v)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "lcache: could not scan in iter, %v", err)
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
func evict(r query, key string, tbl string) (KV, error) {
	var value []byte
	err := r.QueryRow(fmt.Sprintf(`
		DELETE FROM %s 
		WHERE key = $1
		RETURNING value
	`, tbl), key).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		return KV{}, NotFound
	}
	if err != nil {
		return KV{}, fmt.Errorf("could not delete key %s, err; %w", key, err)
	}
	return KV{K: key, V: value}, err
}

func get(r query, key string, tbl string) ([]byte, error) {

	do := func() ([]byte, error) {
		var value []byte
		err := r.QueryRow(fmt.Sprintf(`
		SELECT value FROM %s 
		WHERE key = $1 AND strftime('%%s', 'now') < expire_at
	`, tbl), key).Scan(&value)
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

func setTTL(r query, key string, value []byte, ttl time.Duration, tbl string) error {

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
	`, tbl), key, string(value), int(ttl.Seconds()))
		return err
	}

	err := do()

	if err != nil {
		return fmt.Errorf("could not exec, %w", err)
	}
	return nil
}

func batchGet(db query, keys []string, tbl string) ([]KV, error) {
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
	var kvs []KV
	for rows.Next() {
		var key string
		var value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kvs = append(kvs, KV{K: key, V: []byte(value)})
	}
	return kvs, nil
}

func batchSet(db query, rows []KV, ttl time.Duration, tbl string) error {

	var values []string
	var params []any

	for i, kv := range rows {
		values = append(values, fmt.Sprintf("($%d, $%d, strftime('%%s', 'now')+$%d, $%d)", i*3+1, i*3+2, i*3+3, i*3+3))
		params = append(params, kv.K, string(kv.V), int(ttl.Seconds()))
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

func batchEvict(r query, keys []string, tbl string) ([]KV, error) {
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
	var kvs []KV
	for rows.Next() {
		var key string
		var value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kvs = append(kvs, KV{K: key, V: []byte(value)})
	}

	return kvs, nil

}

func vacuum(r query, max int, tbl string) ([]KV, error) {

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
	var kvs []KV
	for rows.Next() {
		var key string
		var value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("could not scan, %w", err)
		}
		kvs = append(kvs, KV{K: key, V: []byte(value)})
	}
	return kvs, nil
}
