package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type Stdout struct {
	writer io.Writer
}

func (s Stdout) Close() error {
	return nil
}

func (s Stdout) SetError(ctx context.Context, ns, code string, err error) {
	s.Error(ctx, ns, code, err)
}

func (s Stdout) ClearError(ctx context.Context, ns, code string, err error) {
	s.Info(ctx, ns, code, err)
}

func (s Stdout) GetPage(page, pageSize int, filters ...Filter) ([]Event, int, error) {
	return []Event{}, 0, nil
}

func New(writer io.Writer) *Stdout {
	return &Stdout{
		writer: writer,
	}
}

func (s Stdout) Log(l *Event) error {
	out, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("could not marshal message: %w", err)
	}
	_, err = fmt.Fprintln(s.writer, string(out))
	return err
}

func (s Stdout) Info(_ context.Context, namespace, code string, payload interface{}) {
	l := get()
	defer collect(l)
	l.Namespace = namespace
	l.Event = code
	l.Payload = payload
	_ = s.Log(l)
}

func (s Stdout) Error(_ context.Context, namespace, code string, payload interface{}) {
	l := get()
	defer collect(l)
	l.Namespace = namespace
	l.Event = code
	l.Payload = payload
	_ = s.Log(l)
}
