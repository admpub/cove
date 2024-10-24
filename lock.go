package lcache

import "sync"

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

type lockEntry struct {
	mu       sync.Mutex
	refCount int
}

func keyedMu() *keyedMutex {
	sl := &keyedMutex{
		locks: make(map[string]*lockEntry),
	}

	return sl
}

func (km *keyedMutex) Locked(key string) bool {
	km.mu.Lock()
	defer km.mu.Unlock()
	le, exists := km.locks[key]
	return exists && le.refCount > 0
}

func (km *keyedMutex) TryLocked(key string) bool {
	km.mu.Lock()
	defer km.mu.Unlock()
	le, exists := km.locks[key]
	if exists && le.refCount > 0 {
		return false
	}
	if !exists {
		le = &lockEntry{}
		km.locks[key] = le
	}
	le.refCount++
	le.mu.Lock()
	return true
}

func (km *keyedMutex) Lock(key string) {
	km.mu.Lock()
	le, exists := km.locks[key]
	if !exists {
		le = &lockEntry{}
		km.locks[key] = le
	}
	le.refCount++
	km.mu.Unlock()

	le.mu.Lock()
}

func (km *keyedMutex) Unlock(key string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	le, exists := km.locks[key]
	if !exists {
		panic("unlock of unlocked lock")
	}
	le.refCount--
	if le.refCount == 0 {
		delete(km.locks, key)
	}
	le.mu.Unlock()
}
