package lrusvc

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

func NewLruCache[K comparable, V any]() *lru.Cache[K, V] {
	cache, _ := lru.New[K, V](100)
	return cache
}
