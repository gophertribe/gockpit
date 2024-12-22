package monitor

import (
	"encoding/json"
	"time"
)

type Time struct {
	time.Time
}

func (t *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Unix())
}

func (t *Time) UnmarshalJSON(data []byte) error {
	var unix int64
	if err := json.Unmarshal(data, &unix); err != nil {
		return err
	}
	t.Time = time.Unix(unix, 0)
	return nil
}

type Alert struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	First       Time        `json:"first"`
	Latest      Time        `json:"latest"`
	Ack         bool        `json:"ack"`
	SnoozeUntil Time        `json:"snooze_until"`
}
