package audit

import "context"

type Logger interface {
	Error(ctx context.Context, ns, code string, payload interface{})
	Info(ctx context.Context, ns, event string, payload interface{})
	GetPage(page, pageSize int, filters ...Filter) ([]Event, int, error)
	SetError(ctx context.Context, ns, code string, err error)
	ClearError(ctx context.Context, ns, code string, err error)
}
