package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StorageAdapter exists so a future S3-compatible backend just means implementing this
// interface — swap the implementation, not the calling code. LocalDiskAdapter is the only
// implementation here so far.
type StorageAdapter interface {
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

type LocalDiskAdapter struct {
	root string
}

func NewLocalDiskAdapter(dir string) (*LocalDiskAdapter, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving media storage dir: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating media storage dir: %w", err)
	}
	return &LocalDiskAdapter{root: root}, nil
}

func (a *LocalDiskAdapter) resolve(key string) (string, error) {
	full, err := filepath.Abs(filepath.Join(a.root, filepath.FromSlash(key)))
	if err != nil {
		return "", err
	}
	if full != a.root && !strings.HasPrefix(full, a.root+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid storage key: %s", key)
	}
	return full, nil
}

func (a *LocalDiskAdapter) Put(key string, data []byte) error {
	full, err := a.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (a *LocalDiskAdapter) Get(key string) ([]byte, error) {
	full, err := a.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (a *LocalDiskAdapter) Delete(key string) error {
	full, err := a.resolve(key)
	if err != nil {
		return err
	}
	return os.Remove(full)
}
