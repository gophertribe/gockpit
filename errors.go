package gockpit

import (
	"encoding/json"
	"strings"
	"time"
)

type Error struct {
	Err          error
	Count        int
	LastOccurred time.Time
}

func (e Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Error        string    `json:"error"`
		Count        int       `json:"count"`
		LastOccurred time.Time `json:"lastOccur"`
	}{e.Err.Error(), e.Count, e.LastOccurred})
}

func (e Error) Error() string {
	return e.Err.Error()
}

type Errors map[string]Error

func (e Errors) Error() string {
	var build strings.Builder
	for _, err := range e {
		build.WriteString(err.Error())
		build.WriteRune('\n')
	}
	return build.String()
}

func (e Errors) Collect(code string, err error) {
	existing, ok := e[code]
	if !ok {
		e[code] = Error{Err: err, Count: 1, LastOccurred: time.Now()}
		return
	}
	existing.Count++
	existing.LastOccurred = time.Now()
	existing.Err = err // set to latest occurrence as several errors may share the same id
	e[code] = existing
}
