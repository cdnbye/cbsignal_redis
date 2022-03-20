package smap

import (
	"cbsignal/client"
	"github.com/zhangyunhao116/skipmap"
)

type SkipMap struct {
	m *skipmap.StringMap
}

func NewSMap() SkipMap {
	s := SkipMap{
		m: skipmap.NewString(),
	}
	return s
}

func (s SkipMap)CountNoLock() int {
	return s.m.Len()
}

func (s SkipMap)CountPerMapNoLock() []int {
	return []int{s.m.Len()}
}

func (s SkipMap)Set(key string, value *client.Client) {
	s.m.Store(key, value)
}

func (s SkipMap)Get(key string) (*client.Client, bool) {
	v, ok := s.m.Load(key)
	if !ok {
		return nil, false
	}
	return v.(*client.Client), true
}

func (s SkipMap)Has(key string) bool {
	_, ok := s.m.Load(key)
	return ok
}

func (s SkipMap)Remove(key string) {
	s.m.Delete(key)
}

func (s SkipMap)Clear() {
	s.m.Range(func(key string, value interface{}) bool {
		s.m.Delete(key)
		return true
	})
}

func (s SkipMap) Range(f func(key string, value *client.Client) bool) {
	s.m.Range(func(key string, value interface{}) bool {
		return f(key, value.(*client.Client))
	})
}
