package main

import "sync"

type memberModel struct {
	body map[string]bool
	arr  []string
	sync.Mutex
}

func (m *memberModel) Reset() {
	m.Lock()
	defer m.Unlock()
	m.body = make(map[string]bool)
	m.arr = make([]string, 0)
}

func (m *memberModel) Add(v string) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.body[v]; !ok {
		m.body[v] = true
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
