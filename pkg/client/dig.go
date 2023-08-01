package client

import (
	"aliyun-images-syncer/dep"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.uber.org/dig"
)

func DIDependency() (out *Dependency) {
	container := dep.DI()
	if err := container.Invoke(func(dep Dependency) { out = &dep }); err != nil {
		panic(err)
	}

	return
}

type Dependency struct {
	dig.In

	Lru *lru.Cache[any, any]
}
