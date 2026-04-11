package store

import (
	"sync"
	"time"
)

type Store interface {
	Get(k string) (string, bool)

	Set(k, v string, ttl int64)
}

type store struct {
	data map[string]record
	sync.RWMutex
}

type record struct {
	value string
	timer *time.Timer
}

func New() Store {
	return &store{data: make(map[string]record)}
}

func (s *store) Get(k string) (string, bool) {
	s.RLock()
	defer s.RUnlock()

	r, ok := s.data[k]
	return r.value, ok
}

func (s *store) Set(k, v string, ttl int64) {
	s.Lock()
	defer s.Unlock()

	var timer *time.Timer

	// check if key already exists and stop previous timer
	if r, ok := s.data[k]; ok {
		if r.timer != nil {
			r.timer.Stop()
		}
	}

	if ttl > 0 {
		timer = time.AfterFunc(time.Duration(ttl)*time.Millisecond, s.removeKey(k))
	}

	r := record{
		value: v,
		timer: timer,
	}

	s.data[k] = r
}

func (s *store) removeKey(key string) func() {
	return func() {
		s.Lock()
		defer s.Unlock()

		delete(s.data, key)
	}
}
