package state

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mklimuk/gockpit"

	"github.com/go-chi/chi"
)

type ErrorCollector interface {
	Collect(ctx context.Context, ns, code, msg string, cause error, flags ...Flag) error
}

type Flag int

func (f Flag) Is(check Flag) bool {
	return f&check > 0
}

const (
	Clearable Flag = 1 << iota
	Fatal
)

func init() {
	gob.Register(&Error{})
}

type ErrorHandler interface {
	SetError(ctx context.Context, ns, code string, err error)
	ClearError(ctx context.Context, ns, code string, err error)
}

type ErrorList []Error

func (errs ErrorList) Error() string {
	msg := ""
	for _, e := range errs {
		msg += e.Error()
		msg += "\n"
	}
	return msg
}

type Error struct {
	Msg           string    `json:"msg"`
	Clearable     bool      `json:"clearable"`
	Fatal         bool      `json:"fatal"`
	FirstOccurred time.Time `json:"first_occurred"`
	LastOccurred  time.Time `json:"last_occurred"`
	Count         int       `json:"count"`
	flags         Flag
	cause         error
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %v", e.Msg, e.cause)
}

type Errors struct {
	errs       map[string]map[string]Error
	downstream []ErrorHandler
}

func NewErrors(downstream ...ErrorHandler) *Errors {
	return &Errors{
		errs:       make(map[string]map[string]Error),
		downstream: downstream,
	}
}

func (e *Errors) AddDownstream(h ErrorHandler) {
	e.downstream = append(e.downstream, h)
}

func (e *Errors) Collect(ctx context.Context, namespace, code, msg string, err error, flags ...Flag) error {
	var flag Flag
	for _, f := range flags {
		flag |= f
	}
	if err == nil {
		e.Clear(ctx, namespace, code)
		return err
	}
	ns := e.errs[namespace]
	if ns == nil {
		ns = make(map[string]Error)
		e.errs[namespace] = ns
	}
	er, exists := ns[code]
	if !exists {
		er = Error{
			Clearable:     flag.Is(Clearable),
			FirstOccurred: time.Now(),
		}
	}
	// this allows error to become fatal with time (i.e. after several occurrences)
	er.Fatal = flag.Is(Fatal)
	er.Msg = msg
	er.cause = err
	er.Count++
	er.LastOccurred = time.Now()
	er.flags = flag
	ns[code] = er
	for _, h := range e.downstream {
		h.SetError(ctx, namespace, code, er)
	}
	return err
}

func (e *Errors) Clear(ctx context.Context, namespace, code string) {
	ns := e.errs[namespace]
	if ns == nil {
		return
	}
	set, found := ns[code]
	if !found || !set.Clearable {
		return
	}
	delete(ns, code)
	for _, h := range e.downstream {
		h.ClearError(ctx, namespace, code, set)
	}
}

func (e *Errors) Empty() bool {
	return len(e.errs) == 0
}

func (e *Errors) Error() string {
	var err strings.Builder
	for ns, errs := range e.errs {
		err.WriteString(fmt.Sprintf("[%s]\n", ns))
		for code, i := range errs {
			err.WriteString(fmt.Sprintf("%s: %s", code, i.Msg))
		}
	}
	return err.String()
}

func (e *Errors) GetAllByFlag(f Flag) ErrorList {
	var res []Error
	for _, ns := range e.errs {
		for _, err := range ns {
			if err.flags&f > 0 {
				res = append(res, err)
			}
		}
	}
	return res
}

func (e *Errors) Get(ns string, code string) Error {
	namespace := e.errs[ns]
	if namespace == nil {
		return Error{}
	}
	return namespace[code]
}

func GetErrorsHandler(errs *Errors, logger gockpit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, errs, logger)
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}, logger gockpit.Logger) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		logger.Errorf("could not encode body: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(buf.Bytes())
	if err != nil {
		logger.Errorf("could not write response: %w", err)
	}
}

func ClearErrorHandler(errs *Errors) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns := chi.URLParam(r, "namespace")
		code := chi.URLParam(r, "code")
		errs.Clear(r.Context(), ns, code)
		w.WriteHeader(http.StatusOK)
	}
}
