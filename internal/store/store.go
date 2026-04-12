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
	LLen(k string) int
	LPop(k string, length int) []string
	BLPop(k string, timeout float64) []string
}

type store struct {
	kv        map[string]record
	kvList    map[string][]string
	listeners map[string][]chan string
	sync.RWMutex
}

type record struct {
	value string
	timer *time.Timer
}

func New() Store {
	return &store{
		kv:        make(map[string]record),
		kvList:    make(map[string][]string),
		listeners: make(map[string][]chan string),
	}
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

	s.ensureList(k)

	s.kvList[k] = append(s.kvList[k], v...)

	// check listeners and fire
	if listeners, exist := s.listeners[k]; exist {
		n := min(len(v), len(listeners))
		listenersCopy := make([]chan string, n)
		copy(listenersCopy, listeners[:n])

		go func(listenersCopy []chan string) {
			for _, listener := range listenersCopy {
				listener <- "notify"
			}
		}(listenersCopy)
	}

	return len(s.kvList[k])
}

// LPush appends elemens at the left (beginning) end of the list
func (s *store) LPush(k string, v []string) int {
	slices.Reverse(v)
	s.Lock()
	defer s.Unlock()

	s.ensureList(k)

	s.kvList[k] = append(v, s.kvList[k]...)

	if listeners, exist := s.listeners[k]; exist {
		n := min(len(v), len(listeners))
		listenersCopy := make([]chan string, n)
		copy(listenersCopy, listeners[:n])

		go func(listenersCopy []chan string) {
			for _, listener := range listenersCopy {
				listener <- "notify"
			}
		}(listenersCopy)
	}

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

func (s *store) LLen(k string) int {
	s.RLock()
	defer s.RUnlock()

	list, exist := s.kvList[k]

	if !exist {
		return 0
	}
	return len(list)
}

// LPop removes an element from the left (beginning) of the list
func (s *store) LPop(k string, length int) []string {
	s.Lock()
	defer s.Unlock()

	list, exist := s.kvList[k]
	if !exist || len(list) == 0 {
		return nil
	}

	if length > len(list) {
		length = len(list)
	}

	items := list[:length]
	s.kvList[k] = list[length:]

	if len(s.kvList[k]) == 0 {
		delete(s.kvList, k)
	}

	return items
}

func (s *store) BLPop(k string, timeout float64) []string {
	s.Lock()

	_, exist := s.kvList[k]
	if exist {
		s.Unlock()
		return s.LPop(k, 1)
	}

	// check a listener and add to queue so it gets served first
	listener := s.createListener(k)
	s.Unlock()

	var poppedItems []string

	if timeout == 0 {
		<-listener
		poppedItems = s.LPop(k, 1)
	} else {
		select {
		case <-listener:
			poppedItems = s.LPop(k, 1)
		case <-time.After(time.Duration(timeout) * time.Second):
		}
	}

	s.Lock()
	s.removeListener(k, listener)
	s.Unlock()

	if poppedItems == nil {
		return nil
	}

	return []string{k, poppedItems[0]}
}

func (s *store) ensureList(k string) {
	if _, exist := s.kvList[k]; !exist {
		s.kvList[k] = []string{}
	}
}

func (s *store) createListener(k string) chan string {
	listener := make(chan string)
	if _, exist := s.listeners[k]; exist {
		s.listeners[k] = append(s.listeners[k], listener)
		return listener
	}

	s.listeners[k] = []chan string{listener}

	return listener
}

func (s *store) removeListener(k string, currListener chan string) {
	listeners := make([]chan string, 0, len(s.listeners[k]))
	for _, listener := range s.listeners[k] {
		if listener != currListener {
			listeners = append(listeners, listener)
		}
	}

	s.listeners[k] = listeners
	if len(s.listeners[k]) == 0 {
		delete(s.listeners, k)
	}

	close(currListener)
}

func (s *store) removeKey(key string) func() {
	return func() {
		s.Lock()
		defer s.Unlock()

		delete(s.kv, key)
	}
}
