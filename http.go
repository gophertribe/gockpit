package gockpit

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type HandlerError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func RenderJSON(w http.ResponseWriter, status int, body interface{}) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
