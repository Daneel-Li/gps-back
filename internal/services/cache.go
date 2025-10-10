package services

import (
	"encoding/gob"
	"log/slog"
	"os"
	"sync"
	"time"
)

type shard struct { // Cache shard
	mu       sync.RWMutex
	Items    map[string]interface{}
	filepath string
	updated  bool // Whether updated
}

type LocalCache interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)

	//TODO Change to arbitrary multi-level keys
	GetCache(class string, key string) (interface{}, bool)
	SetCache(class string, key string, value interface{})
}

/*
*
If filepath is empty, this cache does not have disk serialization capability
*/
func NewLocalCache(filepath string) LocalCache {
	gob.Register(shard{})
	s := &shard{
		mu:    sync.RWMutex{},
		Items: make(map[string]interface{}),
	}
	if filepath != "" {
		s.filepath = filepath
		if err := s.loadCacheFromFile(filepath); err != nil { // New structure read failed, try old structure
			slog.Info("Cache serialization failed, trying old structure", "error", err)
			if err := s.loadCacheFromJSON(); err != nil {
				slog.Info("Old structure cache serialization also failed, returning empty cache", "error", err)
			} else {
				slog.Info("Successfully used old structure cache serialization")
			}
		}
		s.toPointer(true)
	}

	go func() { // Timed file storage
		if s.filepath == "" {
			return
		}
		for {
			<-time.After(time.Second * 10) // Save every 10 seconds
			s.saveCacheToFile()
		}
	}()
	return s
}
func (s *shard) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Items[key] = value
}
func (s *shard) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.Items[key]
	return res, ok
}

func RegisterStruct(s interface{}) {
	gob.Register(s)
}

// Custom decode function
func (s *shard) loadCacheFromJSON() error {

	// Method 1: Use ioutil.ReadFile (Go 1.15 and earlier versions)

	// First decode to temporary type
	var temp map[string]map[string]interface{}
	file, _ := os.Open(s.filepath)
	defer file.Close()
	if err := gob.NewDecoder(file).Decode(&temp); err != nil {
		return err
	}

	// Convert to target type
	for key, itemMap := range temp {
		shard := &shard{
			Items: make(map[string]interface{}),
		}
		for k, v := range itemMap {
			shard.Items[k] = v
		}
		s.Set(key, shard)
	}

	return nil
}

// Load cache data from file
func (s *shard) loadCacheFromFile(filepath string) error {

	file, _ := os.Open(filepath)
	defer file.Close()
	err := gob.NewDecoder(file).Decode(s)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Info("Failed to read cache file", "error", err)
		}

		return err
	}
	return nil
}

// Convert cache data to pointer type for easy serialization
func (s *shard) toPointer(recursive bool) *shard {
	if recursive {
		for k, v := range s.Items {
			if ss, ok := v.(shard); ok {
				s.Set(k, ss.toPointer(recursive))
			}
			// else { // Other structs are not recursive, handled by user
			// 	val := reflect.ValueOf(v)
			// 	if val.Kind() == reflect.Struct {
			// 		ptr := reflect.New(val.Type())
			// 		ptr.Elem().Set(val)
			// 		s.Set(k, ptr.Interface())
			// 	}
			// }
		}
	}
	return s
}

// Save cache to file
func (s *shard) saveCacheToFile() {
	// Use write lock instead of read lock here, because if read lock is used, other writes may read content,
	// User might store pointers, and modifying pointer content after getting pointer will also modify cache content,
	// When gob serializes, it modifies the content pointed by pointer instead of pointer itself, causing conflicts
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.updated {
		return
	}
	defer func() {
		s.updated = false
	}()
	slog.Debug("Save cache to file", "filepath", s.filepath)
	// Using write lock cannot completely eliminate the above conflicts, just reduce them. Few cases will still conflict, for example if user has a very long operation,
	// It gets the pointer before this write lock is acquired, and continues to operate on that pointer for a relatively long time afterwards
	// TODO Change to redis later, no need to over-design

	file, _ := os.Create(s.filepath)
	defer file.Close()
	err := gob.NewEncoder(file).Encode(s)
	if err != nil {
		slog.Error("Cache serialization failed", "error", err)
		return
	}

}

func (s *shard) GetSubCache(class string) *shard {
	var ss *shard = nil
	if v, ok := s.Get(class); ok {
		ss, _ = v.(*shard)
	}
	if ss == nil {
		ss = &shard{Items: make(map[string]interface{})}
		s.Set(class, ss)
	}
	return ss
}

func (s *shard) SetCache(class string, key string, value interface{}) {
	var ss = s.GetSubCache(class)
	ss.Set(key, value)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updated = true
}

// TODO: Introduce redis and other cache tools later
func (s *shard) GetCache(class string, key string) (interface{}, bool) {
	var ss *shard = s.GetSubCache(class)
	return ss.Get(key)
}
