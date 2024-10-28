package cove

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Vacuum(interval time.Duration, max int) func(cache *Cache) {
	return func(cache *Cache) {

		vacuumNamespace := func(ns *Cache) int {
			start := time.Now()
			n, err := ns.Vacuum(max)
			elapsed := time.Since(start)
			if err != nil {
				cache.log.Error("could not vacuum namespace", "err", err)
				return 0
			}
			if cache.log != nil && n > 0 {
				cache.log.Debug("[cove] vacuumed", "ns", ns.namespace, "time", elapsed, "n", n)
			}
			if cache.log != nil && n == 0 {
				cache.log.Debug("[cove] vacuumed", "ns", ns.namespace, "time", elapsed, "n", n)
			}
			return n

		}

		tic := time.NewTicker(interval)
		for {
			select {
			case <-tic.C:
			case <-cache.closed:
				cache.log.Info("[cove] vacuum stopping")
				return
			}
			for _, namespace := range cache.namespaces {
				start := time.Now()
				for i := 0; i < 100; i++ {

					n := vacuumNamespace(namespace)

					if n == 0 {
						break
					}

					elapsed := time.Since(start)
					if elapsed > time.Second {
						break
					}

					select {
					case <-cache.closed:
						cache.log.Info("[cove] vacuum stopping")
						return
					default:
					}

				}
			}
		}
	}
}

func URITemp() string {
	d := fmt.Sprintf("%d-cove", time.Now().Unix())
	uri := filepath.Join(os.TempDir(), d, "cove.db")
	os.MkdirAll(filepath.Dir(uri), 0755)
	return fmt.Sprintf("file:%s?tmp=true", uri)
}

func URIFromPath(path string) string {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return fmt.Sprintf("file:%s", path)
}

func Hit(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, NotFound) {
		return false, nil
	}
	return false, err
}

func Miss(err error) (bool, error) {
	hit, err := Hit(err)
	return !hit, err
}
