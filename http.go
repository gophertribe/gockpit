package gockpit

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

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

type EventPublisher struct {
	mx          sync.Mutex
	connections map[string]*Conn
	sup         *Supervisor
}

func NewEventPublisher(sup *Supervisor) *EventPublisher {
	e := &EventPublisher{
		sup:         sup,
		connections: map[string]*Conn{},
	}
	sup.AddListener(e.publishState)
	return e
}

//Events streams
func (pub *EventPublisher) EventsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			_ = writeJSONResponse(w, http.StatusInternalServerError, struct {
				Error string `json:"error"`
			}{err.Error()})
			return
		}
		prev := pub.connections[r.RemoteAddr]
		if prev != nil {
			err = prev.ws.Close(websocket.StatusGoingAway, "received another connection from peer")
			if err != nil {
				log.Warn().Err(err).Str("peer", r.RemoteAddr).Msg("could not close previous connection from peer")
			}
		}
		conn := NewConn(r.RemoteAddr, ws)
		pub.mx.Lock()
		pub.connections[r.RemoteAddr] = conn
		pub.mx.Unlock()
		pub.writeState(r.Context(), r.RemoteAddr, pub.sup.GetState(), conn, nil)
	}
}

func writeJSONResponse(w http.ResponseWriter, code int, resp interface{}) error {
	enc, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-MsgType", JSONContentType)
	w.WriteHeader(code)

	_, err = w.Write(enc)
	if err != nil {
		return err
	}
	return nil
}

func (pub *EventPublisher) StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = writeJSONResponse(w, http.StatusOK, pub.connections)
	}
}

func (pub *EventPublisher) writeState(ctx context.Context, peer string, state *State, conn *Conn, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	err := wsjson.Write(ctx, conn.ws, state)
	if err != nil {
		status := websocket.CloseStatus(err)
		if status != websocket.StatusGoingAway && status != websocket.StatusNormalClosure {
			log.Warn().Err(err).Str("peer", peer).Int("status", int(status)).Msg("could not write state to websocket; closing connection")
		}
		log.Info().Str("peer", peer).Int("status", int(status)).Msg("closing peer connection")
		_ = conn.ws.Close(websocket.StatusAbnormalClosure, "error writing state")
		pub.mx.Lock()
		delete(pub.connections, peer)
		pub.mx.Unlock()
		return
	}
	log.Info().Str("peer", peer).Msg("wrote state to peer")
}

func (pub *EventPublisher) publishState(current *State) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	pub.mx.Lock()
	for peer, conn := range pub.connections {
		wg.Add(1)
		go pub.writeState(ctx, peer, current, conn, &wg)
	}
	pub.mx.Unlock()
	wg.Wait()
}
