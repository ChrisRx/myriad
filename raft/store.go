package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

type Store struct {
	mu sync.RWMutex
	kv map[string]string
}

func (s *Store) Apply(l *raft.Log) interface{} {
	var c Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}
	switch c.Op {
	case Set:
		s.Set(c.Key, c.Value)
		return nil
	case Delete:
		s.Delete(c.Key)
		return nil
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
}

func (s *Store) Restore(rc io.ReadCloser) error {
	o := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}
	s.kv = o
	return nil
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := make(map[string]string)
	for k, v := range s.kv {
		o[k] = v
	}
	return &snapshot{store: o}, nil
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.kv[key]; ok {
		return val, ok
	}
	return "", false
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	s.kv[key] = value
	s.mu.Unlock()
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.kv, key)
	s.mu.Unlock()
}

type snapshot struct {
	store map[string]string
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		b, err := json.Marshal(s.store)
		if err != nil {
			return err
		}
		if _, err := sink.Write(b); err != nil {
			return err
		}
		if err := sink.Close(); err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (s *snapshot) Release() {}
