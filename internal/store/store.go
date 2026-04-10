package store

type Store interface {
	Put(k, v string) error
	Get(k string) (string, bool)
}

type store struct {
	data map[string]string
}

func New() Store {
	return &store{data: make(map[string]string)}
}

func (s *store) Put(k, v string) error {
	s.data[k] = v

	return nil
}

func (s *store) Get(k string) (string, bool) {
	val, ok := s.data[k]
	return val, ok
}
