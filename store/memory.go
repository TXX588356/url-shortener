package store

import "sync"

var (
	URLStore = make(map[string]string)
	Mu sync.RWMutex
)