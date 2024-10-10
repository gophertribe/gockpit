package grafana

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"nhooyr.io/websocket"

	"github.com/mklimuk/gockpit"
)

func HandlerDashboards(g *Grafana) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gockpit.RenderJSON(w, http.StatusOK, g.dashboards)
	}
}

func FrontendProxy(prefix string, dashURL *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = dashURL.Scheme
			req.URL.Host = dashURL.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
			req.Header.Add("X-WEBAUTH-USER", "admin")
		},
		ModifyResponse: func(res *http.Response) error {
			res.Header.Del("x-frame-options")
			return nil
		},
	}
}

func WebsocketProxy(ctx context.Context, dashURL *url.URL) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			gockpit.RenderJSON(w, http.StatusInternalServerError, struct {
				Error string `json:"error"`
			}{err.Error()})
			return
		}
		out, _, err := websocket.Dial(ctx, dashURL.String()+"/api/live/ws", &websocket.DialOptions{})
		if err != nil {
			gockpit.RenderJSON(w, http.StatusInternalServerError, struct {
				Error string `json:"error"`
			}{err.Error()})
			return
		}
		// in to out
		go func() {
			for {
				msgType, msg, err := in.Read(ctx)
				if err != nil {
					status := websocket.CloseStatus(err)
					if status != -1 {
						slog.Info("websocket closed with status", "status", status)
					} else {
						slog.Info("websocket error", "err", err)
						status = websocket.StatusAbnormalClosure
					}
					err = out.Close(status, "upstream closed")
					if err != nil {
						slog.Info("could not close proxy websocket", "err", err)
					}
					return
				}
				err = out.Write(ctx, msgType, msg)
				if err != nil {
					slog.Info("could not write message", "err", err)
				}
			}
		}()
		// out to in
		go func() {
			for {
				msgType, msg, err := out.Read(ctx)
				if err != nil {
					status := websocket.CloseStatus(err)
					if status != -1 {
						slog.Info("target websocket closed with status", "status", status)
					} else {
						slog.Info("target websocket error", "err", err)
						status = websocket.StatusAbnormalClosure
					}
					err = in.Close(status, "downstream closed")
					if err != nil {
						slog.Info("could not close proxy websocket", "err", err)
					}
					return
				}
				err = in.Write(ctx, msgType, msg)
				if err != nil {
					slog.Info("could not write message from proxy", "err", err)
				}
			}
		}()
	}
}
