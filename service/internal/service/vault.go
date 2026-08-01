package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type credentialVault struct {
	mu       sync.RWMutex
	dataPath string
	keyPath  string
	key      []byte
	entries  map[string]string
}

func openCredentialVault(dataPath, keyPath string) (*credentialVault, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	vault := &credentialVault{dataPath: dataPath, keyPath: keyPath, key: key, entries: map[string]string{}}
	if err := vault.load(); err != nil {
		return nil, err
	}
	return vault, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	if key, err := os.ReadFile(path); err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("credential key must contain 32 bytes")
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := writeAtomic(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (v *credentialVault) load() error {
	ciphertext, err := os.ReadFile(v.dataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	plaintext, err := decrypt(v.key, ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt credential vault: %w", err)
	}
	if err := json.Unmarshal(plaintext, &v.entries); err != nil {
		return fmt.Errorf("decode credential vault: %w", err)
	}
	return nil
}

func (v *credentialVault) get(username string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	password, ok := v.entries[normalizeUsername(username)]
	return password, ok
}

func (v *credentialVault) put(username, password string) error {
	username = normalizeUsername(username)
	if username == "" || password == "" {
		return errors.New("username and password are required")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	previous, existed := v.entries[username]
	v.entries[username] = password
	if err := v.persistLocked(); err != nil {
		if existed {
			v.entries[username] = previous
		} else {
			delete(v.entries, username)
		}
		return err
	}
	return nil
}

func (v *credentialVault) delete(username string) (bool, error) {
	username = normalizeUsername(username)
	if username == "" {
		return false, errors.New("username is required")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	previous, existed := v.entries[username]
	if !existed {
		return false, nil
	}
	delete(v.entries, username)
	if err := v.persistLocked(); err != nil {
		v.entries[username] = previous
		return false, err
	}
	return true, nil
}

func (v *credentialVault) persistLocked() error {
	plaintext, err := json.Marshal(v.entries)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(v.key, plaintext)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(v.dataPath), 0o700); err != nil {
		return err
	}
	return writeAtomic(v.dataPath, ciphertext, 0o600)
}

func (v *credentialVault) usernames() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	usernames := make([]string, 0, len(v.entries))
	for username := range v.entries {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)
	return usernames
}

func normalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("credential vault is truncated")
	}
	nonce, payload := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, payload, nil)
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".yinqiao-credential-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(contents); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
