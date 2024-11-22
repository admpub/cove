package cove_test

import (
	"math/rand/v2"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/admpub/cove"
	_ "github.com/admpub/cove/driver"
	"github.com/stretchr/testify/assert"
)

func BenchmarkSetParallel(b *testing.B) {

	do := func(op ...cove.Op) func(*testing.B) {
		return func(b *testing.B) {
			cache, err := cove.New(cove.URITemp(), append([]cove.Op{cove.DBRemoveOnClose()}, op...)...)
			assert.NoError(b, err)
			defer cache.Close()

			value := []byte("1")

			var i int64
			b.ResetTimer()
			start := time.Now()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ii := atomic.AddInt64(&i, 1)
					k := strconv.Itoa(int(ii))
					err = cache.Set(k, value)
					assert.NoError(b, err)
				}
			})
			elapsed := time.Since(start)
			b.ReportMetric(float64(i)/elapsed.Seconds(), "insert/sec")
		}
	}

	b.Run("default", do())
	b.Run("sync-off", do(cove.DBSyncOff()))
	b.Run("sync-off+autocheckpoint", do(
		cove.DBSyncOff(),
		cove.DBPragma("mmap_size = 100000000"),
		cove.DBPragma("wal_autocheckpoint = 10000"),
		cove.DBPragma("optimize"),
	))

}

func BenchmarkGetParallel(b *testing.B) {

	do := func(op ...cove.Op) func(*testing.B) {
		return func(b *testing.B) {

			cache, err := cove.New(cove.URITemp(), append([]cove.Op{cove.DBRemoveOnClose()}, op...)...)
			assert.NoError(b, err)
			defer cache.Close()

			for i := 0; i < b.N; i++ {
				key := strconv.Itoa(i)
				value := []byte(key)
				err = cache.Set(key, value)
				assert.NoError(b, err)
			}

			var i int64
			b.ResetTimer()
			start := time.Now()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = atomic.AddInt64(&i, 1)
					k := strconv.Itoa(int(rand.Float64() * float64(b.N)))
					_, err = cache.Get(k)
					assert.NoError(b, err)
				}
			})
			elapsed := time.Since(start)
			b.ReportMetric(float64(i)/elapsed.Seconds(), "reads/sec")
		}
	}

	b.Run("default", do())
	b.Run("sync-off", do(cove.DBSyncOff()))
	b.Run("sync-off+autocheckpoint", do(
		cove.DBSyncOff(),
		cove.DBPragma("mmap_size = 100000000"),
		cove.DBPragma("wal_autocheckpoint = 10000"),
		cove.DBPragma("optimize"),
	))

}

func BenchmarkSetMemParallel(b *testing.B) {

	do := func(valSize int, op ...cove.Op) func(*testing.B) {
		return func(b *testing.B) {
			cache, err := cove.New(cove.URITemp(), append([]cove.Op{cove.DBRemoveOnClose()}, op...)...)
			assert.NoError(b, err)
			defer cache.Close()

			var value = make([]byte, valSize)
			for i := 0; i < len(value); i++ {
				value[i] = byte(rand.Int())
			}

			var i int64
			b.ResetTimer()
			start := time.Now()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ii := atomic.AddInt64(&i, 1)
					k := strconv.Itoa(int(ii))
					err = cache.Set(k, value)
					assert.NoError(b, err)
				}
			})
			elapsed := time.Since(start)
			b.ReportMetric(float64(int(i)*len(value)/1_000_000)/elapsed.Seconds(), "write-mb/sec")
		}
	}

	b.Run("default+0.1mb", do(100_000))
	b.Run("default+1mb", do(1_000_000))
	//b.Run("default+10mb", do(10_000_000))
	b.Run("sync-off+0.1mb", do(100_000, cove.DBSyncOff()))
	b.Run("sync-off+1mb", do(1_000_000, cove.DBSyncOff()))
	//b.Run("sync-off+10mb", do(10_000_000, cove.DBSyncOff()))

}

func BenchmarkSetMem(b *testing.B) {

	do := func(valSize int, op ...cove.Op) func(*testing.B) {
		return func(b *testing.B) {
			cache, err := cove.New(cove.URITemp(), append([]cove.Op{cove.DBRemoveOnClose()}, op...)...)
			assert.NoError(b, err)
			defer cache.Close()

			var value = make([]byte, valSize)
			for i := 0; i < len(value); i++ {
				value[i] = byte(rand.Int())
			}

			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				k := strconv.Itoa(int(i))
				err = cache.Set(k, value)
				assert.NoError(b, err)
			}

			elapsed := time.Since(start)
			b.ReportMetric(float64(b.N*len(value)/1_000_000)/elapsed.Seconds(), "write-mb/sec")
		}
	}

	b.Run("default+0.1mb", do(100_000))
	b.Run("default+1mb", do(1_000_000))
	//b.Run("default+10mb", do(10_000_000))
	b.Run("sync-off+0.1mb", do(100_000, cove.DBSyncOff()))
	b.Run("sync-off+1mb", do(1_000_000, cove.DBSyncOff()))
	//b.Run("sync-off+10mb", do(10_000_000, cove.DBSyncOff()))

}

func BenchmarkGetMemParallel(b *testing.B) {

	do := func(valSize int, op ...cove.Op) func(*testing.B) {
		return func(b *testing.B) {
			cache, err := cove.New(cove.URITemp(), append([]cove.Op{cove.DBRemoveOnClose()}, op...)...)
			assert.NoError(b, err)
			defer cache.Close()

			var value = make([]byte, valSize)
			for i := 0; i < len(value); i++ {
				value[i] = byte(rand.Int())
			}

			for i := 0; i < 30; i++ {
				key := strconv.Itoa(i)
				err = cache.Set(key, value)
				assert.NoError(b, err)
			}

			var i int64
			b.ResetTimer()
			start := time.Now()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ii := atomic.AddInt64(&i, 1) % 30
					k := strconv.Itoa(int(ii))
					_, err = cache.Get(k)
					assert.NoError(b, err, "key %s, N %d", k, b.N)
				}
			})
			elapsed := time.Since(start)
			b.ReportMetric(float64(int(i)*len(value)/1_000_000)/elapsed.Seconds(), "read-mb/sec")
		}
	}

	b.Run("default+0.1mb", do(100_000))
	b.Run("default+1mb", do(1_000_000))
	//b.Run("default+10mb", do(10_000_000))
	b.Run("sync-off+0.1mb", do(100_000, cove.DBSyncOff()))
	b.Run("sync-off+1mb", do(1_000_000, cove.DBSyncOff()))
	//b.Run("sync-off+10mb", do(10_000_000, cove.DBSyncOff()))

}

func BenchmarkGetMem(b *testing.B) {

	do := func(valSize int, op ...cove.Op) func(*testing.B) {
		return func(b *testing.B) {
			cache, err := cove.New(cove.URITemp(), append([]cove.Op{cove.DBRemoveOnClose()}, op...)...)
			assert.NoError(b, err)
			defer cache.Close()

			var value = make([]byte, valSize)
			for i := 0; i < len(value); i++ {
				value[i] = byte(rand.Int())
			}

			for i := 0; i < 30; i++ {
				key := strconv.Itoa(i)
				err = cache.Set(key, value)
				assert.NoError(b, err)
			}

			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				ii := i % 30

				k := strconv.Itoa(int(ii))
				_, err = cache.Get(k)
				assert.NoError(b, err, "key %s, N %d", k, b.N)
			}

			elapsed := time.Since(start)
			b.ReportMetric(float64(b.N*len(value)/1_000_000)/elapsed.Seconds(), "read-mb/sec")
		}
	}

	b.Run("default+0.1mb", do(100_000))
	b.Run("default+1mb", do(1_000_000))
	//b.Run("default+10mb", do(10_000_000))
	b.Run("sync-off+0.1mb", do(100_000, cove.DBSyncOff()))
	b.Run("sync-off+1mb", do(1_000_000, cove.DBSyncOff()))
	//b.Run("sync-off+10mb", do(10_000_000, cove.DBSyncOff()))

}
