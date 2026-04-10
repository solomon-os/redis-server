package store

import (
	"sync"
	"time"
)

type Store interface {
	Set(k, v string) error
	Get(k string) (string, bool)

	SetWithArgs(k, v string, ttl int64)
}

type store struct {
	data map[string]string
	sync.RWMutex
}

func New() Store {
	return &store{data: make(map[string]string)}
}

func (s *store) Set(k, v string) error {
	s.Lock()
	defer s.Unlock()

	s.data[k] = v
	return nil
}

func (s *store) Get(k string) (string, bool) {
	s.RLock()
	defer s.RUnlock()

	val, ok := s.data[k]
	return val, ok
}

func (s *store) SetWithArgs(k, v string, ttl int64) {
	s.Lock()
	defer s.Unlock()

	s.data[k] = v
	go s.removeKey(k, ttl)
}

func (s *store) removeKey(key string, ttl int64) {
	if ttl <= 0 {
		return
	}

	time.Sleep(time.Duration(ttl) * time.Millisecond)

	s.Lock()
	defer s.Unlock()

	delete(s.data, key)
}
