package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type entry struct {
	CreatedAt time.Time       `json:"created_at"`
	Data      json.RawMessage `json:"data"`
}

type Cache struct {
	dir string
}

func New(cacheDir string) (*Cache, error) {
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return nil, err
	}

	return &Cache{dir: cacheDir}, nil
}

func (c *Cache) Get(key string, ttl time.Duration, target any) bool {
	data, err := os.ReadFile(c.keyToPath(key))
	if err != nil {
		return false
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return false
	}

	if time.Since(e.CreatedAt) > ttl {
		return false
	}

	return json.Unmarshal(e.Data, target) == nil
}

func (c *Cache) Set(key string, data any) error {
	rawData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	entry := entry{CreatedAt: time.Now(), Data: rawData}
	
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	
	return os.WriteFile(c.keyToPath(key), entryBytes, 0o644)
}

func (c *Cache) keyToPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, fmt.Sprintf("%x.json", hash))
}
