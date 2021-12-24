package hardware

import (
	"net/http"

	"github.com/mklimuk/gockpit"
)

func StateHandler(m *Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gockpit.RenderJSON(w, http.StatusOK, m.state)
	}
}
