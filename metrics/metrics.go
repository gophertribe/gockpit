package metrics

import "context"

type Provider interface {
	GetMetrics(context.Context) (fields map[string]interface{}, tags map[string]string)
}

type Publisher interface {
	Publish(context.Context, Metrics) error
}

type Metrics struct {
	Namespace string                 `json:"namespace"`
	Event     string                 `json:"event"`
	Fields    map[string]interface{} `json:"fields"`
	Tags      map[string]string      `json:"tags,omitempty"`
}

func New(ns string, fields map[string]interface{}, tags map[string]string) Metrics {
	return Metrics{
		Namespace: ns,
		Event:     "metrics",
		Fields:    fields,
		Tags:      tags,
	}
}
