package main

import "sync"

type memberModel struct {
	body map[int64]bool
	arr  []string
	sync.Mutex
}

func (m *memberModel) Reset() {
	m.Lock()
	defer m.Unlock()
	m.body = make(map[int64]bool)
	m.arr = make([]string, 0)
}

func (m *memberModel) Add(id int64, v string) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.body[id]; !ok {
		m.body[id] = true
		m.arr = append(m.arr, v)
	}
}

func (m *memberModel) Pick() []string {
	m.Lock()
	defer m.Unlock()
	arr := m.arr
	m.arr = make([]string, 0)
	return arr
}
