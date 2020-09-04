package influx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	influxdb "github.com/influxdata/influxdb-client-go"
	"github.com/spf13/afero"
)

var fs afero.Fs

func init() {
	fs = afero.NewOsFs()
}

type Store struct {
	mx            sync.Mutex
	client        *influxdb.Client
	metrics       []influxdb.Metric
	sendInterval  time.Duration
	bufferSize    int
	inopts        []influxdb.Option
	org           string
	bucket        string
	tokenLocation string
	opts          Options
	cancelSend    func()
}

type Options struct {
	Username        string
	Password        string
	Token           string
	RetentionPeriod int
	Retry           int
}

type Option func(*Store)

func WithOptions(opts Options) Option {
	return func(store *Store) {
		store.opts = opts
		store.inopts = []influxdb.Option{influxdb.WithUserAndPass(store.opts.Username, store.opts.Password)}
	}
}

func WithBufferSize(size int) Option {
	return func(store *Store) {
		store.bufferSize = size
	}
}

func WithSendInterval(interval time.Duration) Option {
	return func(store *Store) {
		store.sendInterval = interval
	}
}

func NewStore(addr, org, bucket, tokenLocation string, opts ...Option) (*Store, error) {
	s := &Store{
		tokenLocation: tokenLocation,
		org:           org,
		bucket:        bucket,
		bufferSize:    128,
		sendInterval:  30 * time.Second,
	}
	for _, o := range opts {
		o(s)
	}
	token := s.readToken()
	var err error
	s.client, err = influxdb.New(addr, token, s.inopts...)
	if err != nil {
		return s, fmt.Errorf("could not initialize influxdb client: %w", err)
	}
	for i := 0; i < s.opts.Retry; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.client.Ping(ctx)
		cancel()
		if err == nil {
			break
		}
		log.Info().Err(err).Msgf("could not join influx db; waiting for 5s")
		time.Sleep(5 * time.Second)
	}

	if token == "" {
		// try options
		token = s.opts.Token
	}
	if token == "" {
		if len(opts) == 0 {
			return s, errors.New("options are required when no token is provided")
		}
		log.Info().Msg("setting up influx database")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := s.client.Setup(ctx, s.bucket, s.org, s.opts.RetentionPeriod)
		if err != nil {
			return s, fmt.Errorf("could not setup influx database: %w", err)
		}
		if res.Auth.Token != "" {
			err := s.saveToken(res.Auth.Token)
			if err != nil {
				log.Error().Err(err).Msgf("could not save influx token [%s]", res.Auth.Token)
			}
		}
	}
	go s.sendLoop()
	return s, nil
}

func (s *Store) Close() {
	_ = s.client.Close()
	if s.cancelSend != nil {
		s.cancelSend()
	}
}

func (s *Store) Save(ctx context.Context, bucket, name string, fields map[string]interface{}, tags map[string]string) error {
	s.metrics = append(s.metrics, influxdb.NewRowMetric(fields, name, tags, time.Now()))
	if s.bufferSize == 0 || len(s.metrics) == s.bufferSize {
		err := s.sendAndRelease()
		if err != nil {
			return fmt.Errorf("could not save existing measurements: %w", err)
		}
	}
	return nil
}

func (s *Store) readToken() string {
	tok, err := afero.ReadFile(fs, s.tokenLocation)
	if err != nil && err != afero.ErrFileNotFound {
		log.Warn().Err(err).Msg("could not read token file")
	}
	return strings.TrimSpace(string(tok))
}

func (s *Store) saveToken(token string) error {
	file, err := fs.OpenFile(s.tokenLocation, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not open token file: %w", err)
	}
	defer file.Close()
	_, err = file.Write([]byte(token))
	if err != nil {
		return fmt.Errorf("could not write token file: %w", err)
	}
	return nil
}

func (s *Store) sendAndRelease() error {
	s.mx.Lock()
	defer s.mx.Unlock()
	if len(s.metrics) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := s.client.Write(ctx, s.bucket, s.org, s.metrics...)
		if err != nil {
			return fmt.Errorf("could not write metrics: %w", err)
		}
		s.metrics = nil
	}
	return nil
}

func (s *Store) sendLoop() {
	var ctx context.Context
	ctx, s.cancelSend = context.WithCancel(context.Background())
	tick := time.NewTicker(s.sendInterval)
	for {
		select {
		case <-tick.C:
			err := s.sendAndRelease()
			if err != nil {
				log.Error().Err(err).Msg("could not send buffered metrics")
			}
		case <-ctx.Done():
			return
		}
	}
}
