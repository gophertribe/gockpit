package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
)

type Filter func(Event) bool

type Reader interface {
	GetPage(page, pageSize int, filters ...Filter) ([]Event, int, error)
}

type ErrorLogger interface {
	Errorf(string, ...interface{})
}

func GetLogsHandler(reader Reader, logger ErrorLogger) http.HandlerFunc {
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
				}, logger)
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
				}, logger)
				return
			}
		}
		logs, total, err := reader.GetPage(page, pageSize)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, handlerError{
				Error:   "unexpected error",
				Details: err.Error(),
			}, logger)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Logs  []Event `json:"logs"`
			Total int     `json:"total"`
		}{logs, total}, logger)
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}, logger ErrorLogger) {
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

type handlerError struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}
