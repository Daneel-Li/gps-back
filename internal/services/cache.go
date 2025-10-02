package services

import (
	"encoding/gob"
	"log/slog"
	"os"
	"sync"
	"time"
)

type shard struct { //缓存分片
	mu       sync.RWMutex
	Items    map[string]interface{}
	filepath string
	updated  bool //是否更新过
}

type LocalCache interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)

	//TODO 改成任意多级key
	GetCache(class string, key string) (interface{}, bool)
	SetCache(class string, key string, value interface{})
}

/*
*
如果filepath 为空，则该缓存不具备硬盘序列化能力
*/
func NewLocalCache(filepath string) LocalCache {
	gob.Register(shard{})
	s := &shard{
		mu:    sync.RWMutex{},
		Items: make(map[string]interface{}),
	}
	if filepath != "" {
		s.filepath = filepath
		if err := s.loadCacheFromFile(filepath); err != nil { //使用新型结构读取不成功，用旧结构读取
			slog.Info("序列化缓存失败，尝试旧结构", "error", err)
			if err := s.loadCacheFromJSON(); err != nil {
				slog.Info("旧结构序列化缓存也失败,返回空缓存", "error", err)
			} else {
				slog.Info("使用旧结构序列化缓存成功")
			}
		}
		s.toPointer(true)
	}

	go func() { //定时存储文件
		if s.filepath == "" {
			return
		}
		for {
			<-time.After(time.Second * 10) //10 秒保存一次
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

// 自定义解码函数
func (s *shard) loadCacheFromJSON() error {

	// 方法1：使用ioutil.ReadFile (Go 1.15及更早版本)

	// 先解码到临时类型
	var temp map[string]map[string]interface{}
	file, _ := os.Open(s.filepath)
	defer file.Close()
	if err := gob.NewDecoder(file).Decode(&temp); err != nil {
		return err
	}

	// 转换为目标类型
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

// 从文件加载缓存数据
func (s *shard) loadCacheFromFile(filepath string) error {

	file, _ := os.Open(filepath)
	defer file.Close()
	err := gob.NewDecoder(file).Decode(s)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Info("读取缓存文件失败", "error", err)
		}

		return err
	}
	return nil
}

// 将缓存数据转换为指针类型，方便序列化
func (s *shard) toPointer(recursive bool) *shard {
	if recursive {
		for k, v := range s.Items {
			if ss, ok := v.(shard); ok {
				s.Set(k, ss.toPointer(recursive))
			}
			// else { //其它结构体就不递归了，由用户自行处理
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

// 保存缓存到文件
func (s *shard) saveCacheToFile() {
	// 此处使用写锁而非读锁，是因为如果用读锁，其它写成可能读取内容，
	// 用户有可能存的是指针，拿到指针后对指针内容进行修改也会修改到缓存内容，
	// 而gob序列化时，会修改指针指向的内容而非指针，从而产生冲突
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.updated {
		return
	}
	defer func() {
		s.updated = false
	}()
	slog.Debug("保存cache缓存到文件", "filepath", s.filepath)
	//使用写锁也并不能完全杜绝上述冲突，只是减少。少数情况还是会冲突，例如用户如果有某个很长时间的操作，
	//在本写锁获取之前它就获取了指针，并在之后比较长一段时间内也在对该指针进行操作
	// TODO 后续改成redis即可，无需过度设计

	file, _ := os.Create(s.filepath)
	defer file.Close()
	err := gob.NewEncoder(file).Encode(s)
	if err != nil {
		slog.Error("序列化缓存失败", "error", err)
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

// TODO： 后期引入redis等缓存工具
func (s *shard) GetCache(class string, key string) (interface{}, bool) {
	var ss *shard = s.GetSubCache(class)
	return ss.Get(key)
}
