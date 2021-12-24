package state

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/mklimuk/gockpit/audit"

	"github.com/mklimuk/gockpit/log"
	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	var buf bytes.Buffer
	errs := NewErrors(log.NewCombinedLogger(audit.LevelInfo, &buf, audit.Stdout{}, &wsDummy{}))
	_ = errs.Collect("ns", "code", "dummy error", fmt.Errorf("dummy"), true)
	if assert.Len(t, errs.errs, 1) {
		assert.Len(t, errs.errs["ns"], 1)
	}
	assert.Contains(t, buf.String(), "dummy error")
}

type wsDummy struct{}

func (w wsDummy) Publish(state interface{}) error {
	return nil
}
