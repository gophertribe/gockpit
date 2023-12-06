package hardware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mklimuk/gockpit"

	"github.com/nakabonne/tstorage"
)

type MetricsProvider interface {
	GetMetrics(from, to int64) (map[string][]*tstorage.DataPoint, error)
}

func StateHandler(m *Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gockpit.RenderJSON(w, http.StatusOK, m.state)
	}
}

func MetricsHandler(provider MetricsProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fromQuery := r.URL.Query().Get("from")
		toQuery := r.URL.Query().Get("to")
		from := time.Now().Add(-1 * time.Hour).UnixMilli()
		to := time.Now().UnixMilli()

		// defaults to last 1h
		if fromQuery != "" {
			fromTime, err := time.Parse(time.RFC3339, fromQuery)
			if err != nil {
				gockpit.RenderJSON(w, http.StatusBadRequest, gockpit.HandlerError{Error: fmt.Sprintf("could not parse `from`: %s", err.Error())})
				return
			}
			from = fromTime.UnixMilli()
		}
		if toQuery != "" {
			toTime, err := time.Parse(time.RFC3339, toQuery)
			if err != nil {
				gockpit.RenderJSON(w, http.StatusBadRequest, gockpit.HandlerError{Error: fmt.Sprintf("could not parse `to`: %s", err.Error())})
				return
			}
			to = toTime.UnixMilli()
		}
		metrics, err := provider.GetMetrics(from, to)
		if err != nil {
			gockpit.RenderJSON(w, http.StatusInternalServerError, gockpit.HandlerError{Error: fmt.Sprintf("could not get metrics: %s", err.Error())})
			return
		}
		gockpit.RenderJSON(w, http.StatusOK, metrics)
	}
}
