package gockpit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestState_Formats(t *testing.T) {
	s := State{
		data: map[string]interface{}{
			"A": nil,
			"B": 1,
			"C": 5.0,
			"D": "string value",
			"E": true,
			"F": struct{ Complex bool }{true},
			"errors": map[string]error{
				"A": fmt.Errorf("dummy"),
			},
		},
	}
	assert.Equal(t, "", s.String("A"))
	assert.Equal(t, "1", s.String("B"))
	assert.Equal(t, "5", s.String("C"))
	assert.Equal(t, "string value", s.String("D"))
	assert.Equal(t, "true", s.String("E"))
	assert.Equal(t, "{true}", s.String("F"))
	assert.Equal(t, 1, s.Int("B"))
	assert.Equal(t, 5.0, s.Float("C"))
	assert.Equal(t, true, s.Bool("E"))
}

func TestState_Apply(t *testing.T) {
	s1 := &State{
		data: map[string]interface{}{
			"A": nil,
			"C": 5.0,
			"E": true,
			"F": struct{ Complex bool }{true},
			"errors": map[string]error{
				"A": fmt.Errorf("dummy"),
			},
		},
	}
	s2 := &State{
		data: map[string]interface{}{
			"A": "filled",
			"B": 1,
			"D": "string value",
			"E": true,
			"F": struct{ Complex bool }{true},
			"errors": map[string]error{
				"B": fmt.Errorf("dummy"),
			},
		},
	}
	s1.Apply(s2)
	assert.Equal(t, &State{
		data: map[string]interface{}{
			"A": "filled",
			"B": 1,
			"C": 5.0,
			"D": "string value",
			"E": true,
			"F": struct{ Complex bool }{true},
			"errors": map[string]error{
				"B": fmt.Errorf("dummy"),
			},
		},
	}, s1)
}
