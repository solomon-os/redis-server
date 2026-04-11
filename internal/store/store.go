package store

import (
	"slices"
	"sync"
	"time"
)

type Store interface {
	Get(k string) (string, bool)
	Set(k, v string, ttl int64)
	LPush(k string, v []string) int
	RPush(k string, v []string) int
	LRange(k string, start, end int) []string
}

type store struct {
	kv     map[string]record
	kvList map[string][]string
	sync.RWMutex
}

type record struct {
	value string
	timer *time.Timer
}

func New() Store {
	return &store{kv: make(map[string]record), kvList: make(map[string][]string)}
}

func (s *store) Get(k string) (string, bool) {
	s.RLock()
	defer s.RUnlock()

	r, ok := s.kv[k]
	return r.value, ok
}

func (s *store) Set(k, v string, ttl int64) {
	s.Lock()
	defer s.Unlock()

	var timer *time.Timer

	// check if key already exists and stop previous timer
	if r, ok := s.kv[k]; ok {
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

	s.kv[k] = r
}

// RPush appends element at the right end of the array
func (s *store) RPush(k string, v []string) int {
	s.Lock()
	defer s.Unlock()

	if _, exist := s.kvList[k]; !exist {
		s.kvList[k] = v
	}
	s.kvList[k] = append(s.kvList[k], v...)

	return len(s.kvList[k])
}

// LPush appends elemens at the left (beginning) end of the list
func (s *store) LPush(k string, v []string) int {
	slices.Reverse(v)
	s.Lock()
	defer s.Unlock()

	if _, exist := s.kvList[k]; !exist {
		s.kvList[k] = v
		return len(s.kvList[k])
	}
	s.kvList[k] = append(v, s.kvList[k]...)

	return len(s.kvList[k])
}

func (s *store) LRange(k string, start, end int) []string {
	s.RLock()
	defer s.RUnlock()

	list, exist := s.kvList[k]
	if !exist {
		return nil
	}

	if start < 0 {
		start = max(0, len(list)+start)
	}

	if end < 0 {
		end = max(0, len(list)+end)
	}

	if start > end || start >= len(list) {
		return nil
	}

	if end >= len(list) {
		end = len(list) - 1
	}

	return list[start : end+1]
}

func (s *store) removeKey(key string) func() {
	return func() {
		s.Lock()
		defer s.Unlock()

		delete(s.kv, key)
	}
}
