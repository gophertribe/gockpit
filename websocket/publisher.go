package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

type Logger interface {
	Errorf(string, ...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
}

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
	logger      Logger
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

func NewPublisher(logger Logger) *Publisher {
	e := &Publisher{
		connections: map[string]*Conn{},
		logger:      logger,
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
			}{err.Error()}, pub.logger)
			return
		}
		prev := pub.connections[r.RemoteAddr]
		if prev != nil {
			err = prev.ws.Close(websocket.StatusGoingAway, "received another connection from peer")
			if err != nil {
				pub.logger.Infof("could not close previous connection from peer %s", r.RemoteAddr)
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
						pub.logger.Infof("websocket from %s closed with status (%d) %s", addr, ce.Code, ce.Reason)
					case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
						pub.logger.Infof("context is no longer valid: %v", err)
					default:
						pub.logger.Infof("websocket error: %v", err)
					}
					pub.mx.Lock()
					delete(pub.connections, addr)
					pub.mx.Unlock()
					return
				}
				if msg == websocket.MessageBinary {
					pub.logger.Infof("received binary message from %s", addr)
					continue
				}
				var buf bytes.Buffer
				_, _ = io.Copy(&buf, reader)
				pub.logger.Infof("received message from %s: %s", addr, buf.String())
			}
		}(r.RemoteAddr)
		pub.mx.Lock()
		pub.connections[r.RemoteAddr] = conn
		pub.mx.Unlock()
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}, logger Logger) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		logger.Errorf("could not encode body: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}
	w.WriteHeader(status)
	_, err = w.Write(buf.Bytes())
	if err != nil {
		logger.Errorf("could not write response: %w", err)
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
			pub.logger.Infof("could not write state to websocket; closing connection from peer %s: %d", peer, wserr.Code)
			delete(pub.connections, peer)
			return
		}
		_ = conn.ws.Close(websocket.StatusAbnormalClosure, "error writing state")
		delete(pub.connections, peer)
		return
	}
	pub.logger.Debugf("wrote message to peer %s", peer)
}
