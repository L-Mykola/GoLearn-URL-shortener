package memstore

import (
	"context"
	"sync"

	"golearn/internal/link"
)

var _ link.Store = (*Store)(nil)

type Store struct {
	mu    sync.RWMutex
	links map[string]link.Link
}

func New() *Store {
	return &Store{links: make(map[string]link.Link)}
}

func (s *Store) Save(_ context.Context, l link.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.links[l.Code]; ok {
		return link.ErrExists
	}
	s.links[l.Code] = l
	return nil
}

func (s *Store) Get(_ context.Context, code string) (link.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.links[code]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}
	return l, nil
}

func (s *Store) IncrementVisits(_ context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.links[code]
	if !ok {
		return link.ErrNotFound
	}
	l.Visits++
	s.links[code] = l
	return nil
}
