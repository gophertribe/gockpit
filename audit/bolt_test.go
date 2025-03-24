package audit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestBoltWriteOrder(t *testing.T) {
	tmp := os.TempDir()
	file := filepath.Join(tmp, fmt.Sprintf("audit_bolt_%s_test.store", time.Now().Format(time.RFC3339Nano)))
	collect := &out{t: t}
	defer func() { collect.print(os.Stderr) }()
	store, err := NewBolt(file, collect, collect)
	ctx := context.Background()
	require.NoError(t, err)
	require.NoError(t, store.Log(ctx, &Event{Timestamp: time.Now().UnixNano(), ID: "msg1"}))
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, store.Log(ctx, &Event{Timestamp: time.Now().UnixNano(), ID: "msg2"}))
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, store.Log(ctx, &Event{Timestamp: time.Now().UnixNano(), ID: "msg3"}))
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, store.Log(ctx, &Event{Timestamp: time.Now().UnixNano(), ID: "msg4"}))
	time.Sleep(10 * time.Millisecond)
	page, total, err := store.GetPage(1, 10)
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Equal(t, "msg4", page[0].ID)
	assert.Equal(t, uint64(4), page[0].Seq)
	assert.Equal(t, "msg3", page[1].ID)
	assert.Equal(t, uint64(3), page[1].Seq)
	assert.Equal(t, "msg2", page[2].ID)
	assert.Equal(t, uint64(2), page[2].Seq)
	assert.Equal(t, "msg1", page[3].ID)
	assert.Equal(t, uint64(1), page[3].Seq)
}

type out struct {
	messages []interface{}
	buf      bytes.Buffer
	t        require.TestingT
}

func (o *out) Info(msg string) {
	_, err := fmt.Fprintf(&o.buf, msg)
	require.NoError(o.t, err)
}

func (o *out) Infof(msg string, args ...interface{}) {
	_, err := fmt.Fprintf(&o.buf, msg, args...)
	require.NoError(o.t, err)
}

func (o *out) Publish(_ context.Context, msg interface{}) error {
	o.messages = append(o.messages, msg)
	return nil
}

func (o *out) print(out io.Writer) {
	_, err := io.Copy(out, &o.buf)
	require.NoError(o.t, err)
}
