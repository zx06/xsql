package db

import (
	"sync"
)

// Driver 是数据库驱动的最小抽象；第一阶段仅用于注册/发现。
// 后续会扩展为 query/exec 与连接能力。
type Driver interface{}

var (
	mu      sync.RWMutex
	drivers = map[string]Driver{}
)

func Register(name string, d Driver) {
	mu.Lock()
	defer mu.Unlock()
	if name == "" {
		panic("db.Register: empty name")
	}
	if d == nil {
		panic("db.Register: nil driver")
	}
	if _, exists := drivers[name]; exists {
		panic("db.Register: duplicate driver: " + name)
	}
	drivers[name] = d
}

func Get(name string) (Driver, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := drivers[name]
	return d, ok
}

func RegisteredNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(drivers))
	for k := range drivers {
		out = append(out, k)
	}
	return out
}
