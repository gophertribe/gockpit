package state

import (
	"errors"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("state property not found")
var ErrInvalidFormat = errors.New("invalid format")

type State map[string]interface{}

func (s State) String(label string) (string, error) {
	value, found := s[label]
	if !found {
		return "", ErrNotFound
	}
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v), nil
	case string:
		return v, nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', 2, 64), nil
	default:
		return "", ErrInvalidFormat
	}
}

func (s State) Float(label string) (float32, error) {
	value, found := s[label]
	if !found {
		return 0.0, ErrNotFound
	}
	switch v := value.(type) {
	case int:
		return float32(v), nil
	case string:
		return 0.0, ErrInvalidFormat
	case float32:
		return v, nil
	default:
		return 0.0, ErrInvalidFormat
	}
}

func (s State) Bool(label string) (bool, error) {
	value, found := s[label]
	if !found {
		return false, ErrNotFound
	}
	switch v := value.(type) {
	case int:
		return v == 1, nil
	case string:
		return strings.ToLower(v) == "true", ErrInvalidFormat
	case float32:
		return v == 1.0, nil
	default:
		return false, ErrInvalidFormat
	}
}
