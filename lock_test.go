package cove

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyedMutexLocked(t *testing.T) {
	km := keyedMu()
	key := "test-key"

	assert.False(t, km.Locked(key))

	km.Lock(key)
	assert.True(t, km.Locked(key))

	km.Unlock(key)
	assert.False(t, km.Locked(key))
}

func TestKeyedMutexTryLocked(t *testing.T) {
	km := keyedMu()
	key := "test-key"

	assert.True(t, km.TryLocked(key))
	assert.False(t, km.TryLocked(key))

	km.Unlock(key)
	assert.True(t, km.TryLocked(key))
}

func TestKeyedMutexLockUnlock(t *testing.T) {
	km := keyedMu()
	key := "test-key"

	km.Lock(key)
	assert.True(t, km.Locked(key))

	km.Unlock(key)
	assert.False(t, km.Locked(key))
}

func TestKeyedMutexUnlockWithoutLock(t *testing.T) {
	km := keyedMu()
	key := "test-key"

	assert.Panics(t, func() {
		km.Unlock(key)
	})
}

func TestKeyedMutexConcurrentAccess(t *testing.T) {
	km := keyedMu()
	key := "test-key"
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		km.Lock(key)
		defer km.Unlock(key)
	}()

	go func() {
		defer wg.Done()
		km.Lock(key)
		defer km.Unlock(key)
	}()

	wg.Wait()
	assert.False(t, km.Locked(key))
}

func TestKeyedMutexConcurrentAccessMulti(t *testing.T) {
	km := keyedMu()
	key := "test-key"
	var wg sync.WaitGroup

	var size = 1000
	var count int

	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			km.Lock(key)
			defer km.Unlock(key)
			count++
		}()
	}

	wg.Wait()
	assert.False(t, km.Locked(key))
	assert.Equal(t, size, count)
}
