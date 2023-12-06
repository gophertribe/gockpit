package state

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/mklimuk/gockpit/log"
	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.TODO()
	errs := NewErrors(log.NewLeveledLogger(&buf))
	_ = errs.Collect(ctx, "ns", "code", "dummy error", fmt.Errorf("dummy"))
	if assert.Len(t, errs.errs, 1) {
		assert.Len(t, errs.errs["ns"], 1)
	}
	assert.Contains(t, buf.String(), "dummy error")
}

type wsDummy struct{}

func (w wsDummy) Publish(state interface{}) error {
	return nil
}
