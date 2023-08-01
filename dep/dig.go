package dep

import (
	"log"

	"aliyun-images-syncer/service/lrusvc"

	"go.uber.org/dig"
)

// DI 创建容器，注入全局对象
func DI() *dig.Container {
	return NewContainer(
		WithLru(),
	)
}

type Option func(*dig.Container) error

func NewContainer(opts ...Option) *dig.Container {
	container := dig.New()
	for _, opt := range opts {
		if err := opt(container); err != nil {
			log.Fatalf("dig init Container fail: %v", err)
		}
	}
	return container
}

func WithLru() Option {
	return func(c *dig.Container) error {
		return c.Provide(lrusvc.NewLruCache[any, any])
	}
}
