package metrics

import (
	"context"
	"fmt"
	"io"
)

type Store struct {
	out io.Writer
}

func NewMemoryStore(writer io.Writer) *Store {
	return &Store{
		out: writer,
	}
}

func (s *Store) Publish(_ context.Context, m Metrics) error {
	_, err := fmt.Fprintf(s.out, "%s|%s: %+v\n", m.Namespace, m.Event, m.Fields)
	return err
}

func (s *Store) SaveMeasurement(_ context.Context, measurement string, fields map[string]interface{}, _ map[string]string) error {
	_, err := fmt.Fprintf(s.out, "%s: %+v\n", measurement, fields)
	return err
}
