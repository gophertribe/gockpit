package audit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

type Filter func(Event) bool

type Reader interface {
	GetPage(page, pageSize int, filters ...Filter) ([]Event, int, error)
}

func GetLogsHandler(reader Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := 1
		queryPage := r.URL.Query().Get("page")
		var err error
		if queryPage != "" {
			page, err = strconv.Atoi(queryPage)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, handlerError{
					Error:   "invalid `page` param format (expected integer)",
					Details: err.Error(),
				})
				return
			}
		}
		pageSize := 50
		queryPageSize := r.URL.Query().Get("size")
		if queryPageSize != "" {
			pageSize, err = strconv.Atoi(queryPageSize)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, handlerError{
					Error:   "invalid `size` param format (expected integer)",
					Details: err.Error(),
				})
				return
			}
		}
		logs, total, err := reader.GetPage(page, pageSize)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, handlerError{
				Error:   "unexpected error",
				Details: err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Logs  []Event `json:"logs"`
			Total int     `json:"total"`
		}{logs, total})
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		slog.Error("could not encode body", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(buf.Bytes())
	if err != nil {
		slog.Error("could not write response", "error", err)
	}
}

type handlerError struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}
