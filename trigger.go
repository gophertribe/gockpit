package gockpit

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Trigger interface {
	Update(context.Context, *State, []Listener, ReadWriter, *sync.WaitGroup)
}

var (
	_ Trigger = &IntervalTrigger{}
)

type IntervalTrigger struct {
	mx       sync.Mutex
	ID       string
	interval time.Duration
	updates  []ProbeFunc
}

func (i *IntervalTrigger) Update(ctx context.Context, state *State, listeners []Listener, store ReadWriter, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(i.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				i.mx.Lock()
				mutation := state.With()
				for _, up := range i.updates {
					up(ctx, mutation)
				}
				mutation.Apply()
				if mutation.dirty {
					for _, l := range listeners {
						l(state)
					}
				}
				// persist state no matter if it has changed (time series)
				if store != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					state.mx.RLock()
					err := store.Save(ctx, "gockpit", i.ID, state.data, nil)
					state.mx.RUnlock()
					cancel()
					if err != nil {
						log.Error().Err(err).Msg("could not save metrics state")
					}
				}
				i.mx.Unlock()
			case <-ctx.Done():
			}
		}
	}()
}

func NewIntervalTrigger(ID string, interval time.Duration, update ...ProbeFunc) *IntervalTrigger {
	return &IntervalTrigger{
		ID:       ID,
		interval: interval,
		updates:  update,
	}
}
