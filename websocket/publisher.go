package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mklimuk/gockpit"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	JSONContentType = "application/json"
)

type Time struct {
	time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.Format(time.RFC3339))
}

type Conn struct {
	ws    *websocket.Conn
	Peer  string `json:"peer"`
	Since Time   `json:"since"`
}

func NewConn(peer string, conn *websocket.Conn) *Conn {
	return &Conn{
		ws:    conn,
		Peer:  peer,
		Since: Time{time.Now()},
	}
}

type Publisher struct {
	mx          sync.Mutex
	connections map[string]*Conn
	enabled     bool
}

func (pub *Publisher) Publish(ctx context.Context, msg interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	pub.mx.Lock()
	for peer, conn := range pub.connections {
		wg.Add(1)
		go pub.write(ctx, peer, conn, msg, &wg)
	}
	pub.mx.Unlock()
	wg.Wait()
	return nil
}

func NewPublisher() *Publisher {
	e := &Publisher{
		connections: map[string]*Conn{},
	}
	return e
}

// SubscribeHandler streams published events to websockets
func (pub *Publisher) SubscribeHandler(ctx context.Context) http.HandlerFunc {
	pub.enabled = true
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, struct {
				Error string `json:"error"`
			}{err.Error()})
			return
		}
		pub.mx.Lock()
		prev := pub.connections[r.RemoteAddr]
		pub.mx.Unlock()
		if prev != nil {
			err = prev.ws.Close(websocket.StatusGoingAway, "received another connection from peer")
			if err != nil {
				slog.Info("could not close previous connection from peer", "peer", r.RemoteAddr, "error", err)
			}
		}
		conn := NewConn(r.RemoteAddr, ws)
		go func(addr string) {
			for {
				msg, reader, err := ws.Reader(ctx)
				if err != nil {
					var ce websocket.CloseError
					switch {
					case errors.As(err, &ce):
						slog.Info("websocket from peer closed", "peer", addr, "status", ce.Code, "reason", ce.Reason)
					case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
						slog.Info("context is no longer valid", "error", err)
					default:
						slog.Info("websocket error", "error", err)
					}
					pub.mx.Lock()
					delete(pub.connections, addr)
					pub.mx.Unlock()
					return
				}
				if msg == websocket.MessageBinary {
					slog.Info("received binary message from peer", "peer", addr)
					continue
				}
				var buf bytes.Buffer
				_, _ = io.Copy(&buf, reader)
				slog.Info("received message from peer", "peer", addr)
				slog.Debug("message from peer", "peer", addr, "msg", buf.String())
			}
		}(r.RemoteAddr)
		pub.mx.Lock()
		pub.connections[r.RemoteAddr] = conn
		pub.mx.Unlock()
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
	w.WriteHeader(status)
	_, err = w.Write(buf.Bytes())
	if err != nil {
		slog.Error("could not write response", "error", err)
	}
}

func (pub *Publisher) StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gockpit.RenderJSON(w, http.StatusOK, struct {
			Enabled     bool             `json:"enabled"`
			Connections map[string]*Conn `json:"connections"`
		}{
			pub.enabled,
			pub.connections,
		})
	}
}

func (pub *Publisher) write(ctx context.Context, peer string, conn *Conn, msg interface{}, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	err := wsjson.Write(ctx, conn.ws, msg)
	if err != nil {
		var wserr websocket.CloseError
		if errors.As(err, &wserr) {
			slog.Info("could not write state to websocket; closing connection from peer", "peer", peer, "code", wserr.Code)
			delete(pub.connections, peer)
			return
		}
		_ = conn.ws.Close(websocket.StatusAbnormalClosure, "error writing state")
		delete(pub.connections, peer)
		return
	}
	slog.Debug("wrote message to peer", "peer", peer)
}
