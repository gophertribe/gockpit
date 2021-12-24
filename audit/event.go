package audit

import (
	"sync"
)

var pool = sync.Pool{New: func() interface{} {
	return &Event{}
}}

func get() *Event {
	return pool.Get().(*Event)
}

func collect(log *Event) {
	pool.Put(log)
}

type Event struct {
	ID        string      `json:"id"`
	Timestamp int64       `json:"timestamp" storm:"index"`
	Namespace string      `json:"namespace"`
	Level     string      `json:"level"`
	Type      string      `json:"type,omitempty"`
	Event     string      `json:"event"`
	Payload   interface{} `json:"payload,omitempty"`
	Seq       uint64      `json:"seq"`
}
