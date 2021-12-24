package monitor

import (
	"encoding/json"
	"time"
)

type Time struct {
	time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Unix())
}

type Alert struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	First       Time        `json:"first"`
	Latest      Time        `json:"latest"`
	Ack         bool        `json:"ack"`
	SnoozeUntil Time        `json:"snooze_until"`
}
