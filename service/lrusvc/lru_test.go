package lrusvc

import (
	"strconv"
	"testing"
)

func TestLruCache(t *testing.T) {
	cache := NewLruCache[any, any]()

	// Test Add
	for i := 0; i < 100; i++ {
		cache.Add(i, strconv.Itoa(i))
	}

	// Test Get
	for i := 0; i < 100; i++ {
		value, ok := cache.Get(i)
		if !ok || value != strconv.Itoa(i) {
			t.Errorf("Get(%v) = %v, want %v", i, value, strconv.Itoa(i))
		}
	}

	// Test that oldest item is purged
	cache.Add(100, "100")
	_, ok := cache.Get(0)
	if ok {
		t.Error("Expected item 0 to be purged from the cache")
	}

	// Test Contains
	if !cache.Contains(1) {
		t.Error("Expected cache to contain item with key 1")
	}

	// Test Remove
	cache.Remove(1)
	_, ok = cache.Get(1)
	if ok {
		t.Error("Expected item 1 to be removed from the cache")
	}
}
