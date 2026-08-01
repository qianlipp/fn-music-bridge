package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type persistedProtocolSessions struct {
	Version  int               `json:"version"`
	Jellyfin map[string]string `json:"jellyfin"`
}

type protocolSessionStore struct {
	mu       sync.RWMutex
	dataPath string
	key      []byte
	data     persistedProtocolSessions
}

func openProtocolSessionStore(dataPath string, key []byte) (*protocolSessionStore, error) {
	store := &protocolSessionStore{
		dataPath: dataPath,
		key:      append([]byte(nil), key...),
		data: persistedProtocolSessions{
			Version:  1,
			Jellyfin: map[string]string{},
		},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *protocolSessionStore) load() error {
	ciphertext, err := os.ReadFile(s.dataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	plaintext, err := decrypt(s.key, ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt protocol sessions: %w", err)
	}
	var data persistedProtocolSessions
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return fmt.Errorf("decode protocol sessions: %w", err)
	}
	if data.Jellyfin == nil {
		data.Jellyfin = map[string]string{}
	}
	data.Version = 1
	s.data = data
	return nil
}

func (s *protocolSessionStore) jellyfinUsername(token string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	username, ok := s.data.Jellyfin[token]
	return username, ok
}

func (s *protocolSessionStore) putJellyfin(token, username string) error {
	username = normalizeUsername(username)
	if token == "" || username == "" {
		return errors.New("Jellyfin token and username are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, existed := s.data.Jellyfin[token]
	s.data.Jellyfin[token] = username
	if err := s.persistLocked(); err != nil {
		if existed {
			s.data.Jellyfin[token] = previous
		} else {
			delete(s.data.Jellyfin, token)
		}
		return err
	}
	return nil
}

func (s *protocolSessionStore) deleteJellyfin(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	username, existed := s.data.Jellyfin[token]
	if !existed {
		return nil
	}
	delete(s.data.Jellyfin, token)
	if err := s.persistLocked(); err != nil {
		s.data.Jellyfin[token] = username
		return err
	}
	return nil
}

func (s *protocolSessionStore) deleteJellyfinUser(username string) error {
	username = normalizeUsername(username)
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := map[string]string{}
	for token, owner := range s.data.Jellyfin {
		if normalizeUsername(owner) == username {
			removed[token] = owner
			delete(s.data.Jellyfin, token)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	if err := s.persistLocked(); err != nil {
		for token, owner := range removed {
			s.data.Jellyfin[token] = owner
		}
		return err
	}
	return nil
}

func (s *protocolSessionStore) persistLocked() error {
	plaintext, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(s.key, plaintext)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.dataPath), 0o700); err != nil {
		return err
	}
	return writeAtomic(s.dataPath, ciphertext, 0o600)
}
