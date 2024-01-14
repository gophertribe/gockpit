package hardware

import (
	"github.com/mklimuk/gockpit"
	"net/http"
)

func StateHandler(m *Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gockpit.RenderJSON(w, http.StatusOK, m.state)
	}
}
