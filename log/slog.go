package log

import (
	"context"
	"fmt"
	"log/slog"
)

type SlogAdapter struct{}

func (s SlogAdapter) Debug(msg string) {
	slog.Debug(msg)
}

func (s SlogAdapter) Debugf(msg string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(msg, args...))
}

func (s SlogAdapter) Info(msg string) {
	slog.Info(msg)
}

func (s SlogAdapter) Infof(msg string, args ...interface{}) {
	slog.Info(fmt.Sprintf(msg, args...))
}

func (s SlogAdapter) Error(msg string) {
	slog.Error(msg)
}

func (s SlogAdapter) Errorf(msg string, args ...interface{}) {
	slog.Error(fmt.Sprintf(msg, args...))
}

func (s SlogAdapter) SetError(_ context.Context, ns, code string, err error) {
	slog.Error("error occurred", "namespace", ns, "code", code, "error", err)
}

func (s SlogAdapter) ClearError(_ context.Context, ns, code string, _ error) {
	slog.Info("error cleared", "namespace", ns, "code", code)
}
