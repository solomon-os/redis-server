package store

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
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
	KeyType(k string) string
	SetStream(k, id string, fields map[string]string) (string, error)
	RangeStream(k string, start, end string) ([]StreamEntry, error)
	RangeStreamBlock(k, start string, timeout float64) ([]StreamEntry, error)
}

type store struct {
	kv              map[string]record
	kvList          map[string][]string
	streams         map[string][]StreamEntry
	listListeners   map[string][]chan string
	streamListeners map[string][]chan StreamEntry
	sync.RWMutex
}

type record struct {
	value string
	timer *time.Timer
}

type streamID struct {
	ms  int
	seq int
}

func (id streamID) String() string {
	return strconv.Itoa(id.ms) + "-" + strconv.Itoa(id.seq)
}

type StreamEntry struct {
	ID     streamID
	Fields map[string]string
}

func (st StreamEntry) FlatFields() []string {
	fields := make([]string, 0, len(st.Fields)*2)

	for k, v := range st.Fields {
		fields = append(fields, k, v)
	}

	return fields
}

var (
	errStreamIDTooSmall = errors.New(
		"The ID specified in XADD is equal or smaller than the target stream top item",
	)
	errStreamIDZero   = errors.New("The ID specified in XADD must be greater than 0-0")
	errStreamIDFormat = errors.New("invalid id format")
	errStreamIDType   = errors.New("invalid id type")
)

func New() Store {
	return &store{
		kv:              make(map[string]record),
		kvList:          make(map[string][]string),
		streams:         make(map[string][]StreamEntry),
		listListeners:   make(map[string][]chan string),
		streamListeners: make(map[string][]chan StreamEntry),
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

	var popped int

	// check for listeners
	listeners, exist := s.listListeners[k]
	if exist {
		for i := 0; i < min(len(listeners), len(v)); i++ {
			listeners[i] <- v[popped]
			popped++
		}
		s.listListeners[k] = slices.Clone(s.listListeners[k][popped:])
		if len(s.listListeners[k]) == 0 {
			delete(s.listListeners, k)
		}
	}

	s.kvList[k] = append(s.kvList[k], v[popped:]...)

	return len(s.kvList[k])
}

// LPush appends elemens at the left (beginning) end of the list
func (s *store) LPush(k string, v []string) int {
	slices.Reverse(v)
	s.Lock()
	defer s.Unlock()

	s.ensureList(k)

	var popped int

	// check for listeners
	listeners, exist := s.listListeners[k]
	if exist {
		for i := 0; i < min(len(listeners), len(v)); i++ {
			listeners[i] <- v[popped]
			popped++
		}
		s.listListeners[k] = slices.Clone(s.listListeners[k][popped:])
		if len(s.listListeners[k]) == 0 {
			delete(s.listListeners, k)
		}
	}

	s.kvList[k] = append(v[popped:], s.kvList[k]...)

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

func (s *store) rangeStreamUnlocked(k string, start, end string) ([]StreamEntry, error) {
	startId, startSeq, _, _, err := parseStreamID(start)
	if start != "0" && start != "-" && startId == 0 && err != nil {
		return nil, err
	}

	endId, endSeq, _, _, err := parseStreamID(end)
	if end != "0" && end != "+" && endId == 0 && err != nil {
		return nil, err
	}

	// * is for the xread command. it tells it to exlude the start id
	if end == "+" || end == "*" {
		endId, endSeq = math.MaxInt, math.MaxInt
	}

	entries := []StreamEntry{}

	if _, exist := s.streams[k]; !exist {
		return entries, nil
	}

	for i := range s.streams[k] {
		stream := s.streams[k][i]
		entry := StreamEntry{
			ID:     stream.ID,
			Fields: stream.Fields,
		}

		if stream.ID.ms > startId && stream.ID.ms < endId {
			entries = append(entries, entry)
			continue
		}

		if stream.ID.ms == startId {
			if end != "*" && stream.ID.seq >= startSeq && stream.ID.seq <= endSeq {
				entries = append(entries, entry)
				continue
			} else if stream.ID.seq > startSeq && stream.ID.seq <= endSeq {
				entries = append(entries, entry)
			}
		}

		if stream.ID.ms == endId && stream.ID.seq >= startSeq && stream.ID.seq <= endSeq {
			entries = append(entries, entry)
		}

	}
	return entries, nil
}

func (s *store) RangeStream(k string, start, end string) ([]StreamEntry, error,
) {
	s.RLock()
	defer s.RUnlock()

	return s.rangeStreamUnlocked(k, start, end)
}

func (s *store) RangeStreamBlock(k, start string, timeout float64) ([]StreamEntry, error) {
	s.Lock()

	if _, exist := s.streams[k]; exist && len(s.streams[k]) > 0 {
		streams, err := s.rangeStreamUnlocked(k, start, "*")

		if len(streams) > 0 {
			s.Unlock()
			return streams, err
		}
	}

	var streams StreamEntry
	var got bool

	listener := s.createStreamListenerUnlocked(k)

	s.Unlock()

	select {
	case streams = <-listener:
		got = true
	case <-time.After(time.Duration(timeout * float64(time.Millisecond))):
		s.Lock()
		select {
		case streams = <-listener:
			got = true
		default:
		}
		s.removeStreamListener(k, listener)
		s.Unlock()
	}

	if !got {
		return nil, nil
	}

	return []StreamEntry{streams}, nil
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

func (s *store) lpopLocked(k string, length int) ([]string, bool) {
	list, ok := s.kvList[k]
	if !ok || len(list) == 0 {
		return nil, false
	}

	if length > len(list) {
		length = len(list)
	}

	items := list[:length]
	s.kvList[k] = list[length:]

	if len(s.kvList[k]) == 0 {
		delete(s.kvList, k)
	}

	return items, true
}

// LPop removes an element from the left (beginning) of the list
func (s *store) LPop(k string, length int) []string {
	s.Lock()
	defer s.Unlock()

	items, _ := s.lpopLocked(k, length)

	return items
}

func (s *store) BLPop(k string, timeout float64) []string {
	s.Lock()

	items, ok := s.lpopLocked(k, 1)
	if ok {
		s.Unlock()
		return []string{k, items[0]}
	}

	listener := s.createListListener(k)
	s.Unlock()

	var item string
	var got bool

	if timeout == 0 {
		item = <-listener
		got = true
	} else {
		select {
		case item = <-listener:
			got = true
		case <-time.After(time.Duration(timeout * float64(time.Second))):

			// acquire lock again and check if any item has been sent after timeout
			s.Lock()
			select {
			case item = <-listener:
				got = true
			default:
			}
			s.removeListListener(k, listener)
			s.Unlock()
		}
	}

	if !got {
		return nil
	}

	return []string{k, item}
}

func (s *store) KeyType(k string) string {
	if _, exist := s.kv[k]; exist {
		return "string"
	}

	if _, exist := s.kvList[k]; exist {
		return "list"
	}

	if _, exist := s.streams[k]; exist {
		return "stream"
	}

	return ""
}

func (s *store) SetStream(k, req string, fields map[string]string) (string, error) {
	s.Lock()
	defer s.Unlock()

	var last *streamID
	if entries := s.streams[k]; len(entries) > 0 {
		last = &entries[len(entries)-1].ID
	}

	id, err := resolveStreamID(req, last)
	if err != nil {
		return "", err
	}

	stream := StreamEntry{ID: id, Fields: fields}

	if listeners, exist := s.streamListeners[k]; exist {
		for i := range listeners {
			listeners[i] <- stream
		}
		delete(s.streamListeners, k)
	}

	s.streams[k] = append(s.streams[k], stream)
	return id.String(), nil
}

func (s *store) ensureList(k string) {
	if _, exist := s.kvList[k]; !exist {
		s.kvList[k] = []string{}
	}
}

func (s *store) createListListener(k string) chan string {
	listener := make(chan string, 1)
	if _, exist := s.listListeners[k]; exist {
		s.listListeners[k] = append(s.listListeners[k], listener)
		return listener
	}

	s.listListeners[k] = []chan string{listener}

	return listener
}

func (s *store) removeListListener(k string, currListener chan string) {
	listeners := make([]chan string, 0, len(s.listListeners[k])-1)
	for _, listener := range s.listListeners[k] {
		if listener != currListener {
			listeners = append(listeners, listener)
		}
	}

	s.listListeners[k] = listeners
	if len(s.listListeners[k]) == 0 {
		delete(s.listListeners, k)
	}

	close(currListener)
}

func (s *store) createStreamListenerUnlocked(k string) chan StreamEntry {
	listener := make(chan StreamEntry, 1)
	if _, exist := s.streamListeners[k]; exist {
		s.streamListeners[k] = append(s.streamListeners[k], listener)
		return listener
	}

	s.streamListeners[k] = []chan StreamEntry{listener}

	return listener
}

func (s *store) removeStreamListener(k string, currListener chan StreamEntry) {
	listeners := make([]chan StreamEntry, 0, len(s.streamListeners[k])-1)
	for _, listener := range s.streamListeners[k] {
		if listener != currListener {
			listeners = append(listeners, listener)
		}
	}

	s.streamListeners[k] = listeners
	if len(s.streamListeners[k]) == 0 {
		delete(s.streamListeners, k)
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

func parseStreamID(s string) (ms, seq int, msAuto, seqAuto bool, err error) {
	if s == "*" {
		msAuto = true
		return
	}

	parts := strings.SplitN(s, "-", 2)

	ms, err = strconv.Atoi(parts[0])
	if err != nil {
		err = errStreamIDType
		return
	}

	if len(parts) != 2 {
		err = errStreamIDFormat
		return
	}

	if parts[1] == "*" {
		seqAuto = true
		return
	}

	seq, err = strconv.Atoi(parts[1])
	if err != nil {
		err = errStreamIDType
		return
	}

	return
}

func resolveStreamID(req string, last *streamID) (streamID, error) {
	ms, seq, msAuto, seqAuto, err := parseStreamID(req)
	if err != nil {
		return streamID{}, err
	}

	switch {
	case msAuto:
		ms = int(time.Now().UnixMilli())
		if last != nil && last.ms >= ms {
			ms = last.ms
			seq = last.seq + 1
		} else {
			seq = 0
		}
	case seqAuto:
		switch {
		case last == nil:
			if ms == 0 {
				seq = 1
			} else {
				seq = 0
			}
		case ms == last.ms:
			seq = last.seq + 1
		case ms > last.ms:
			seq = 0
		default:
			return streamID{}, errStreamIDTooSmall
		}
	}

	id := streamID{ms: ms, seq: seq}
	if id.ms == 0 && id.seq == 0 {
		return streamID{}, errStreamIDZero
	}
	if last != nil && (id.ms < last.ms || (id.ms == last.ms && id.seq <= last.seq)) {
		return streamID{}, errStreamIDTooSmall
	}
	return id, nil
}
