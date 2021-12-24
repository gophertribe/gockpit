package influx

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mklimuk/gockpit/metrics"

	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/spf13/afero"
)

type line string

type Store struct {
	client    *Client
	writer    api.WriteAPI
	org       string
	bucket    string
	batchSize int
	batch     []line
	logger    Logger
}

func (s *Store) Publish(ctx context.Context, m metrics.Metrics) error {
	err := s.SaveMeasurement(ctx, m.Event, m.Fields, m.Tags)
	if err != nil {
		return fmt.Errorf("could not save `%s` measurement: %w", m.Event, err)
	}
	return nil
}

func (s *Store) GetToken() string {
	return s.client.token
}

type Options struct {
	Username        string
	Password        string
	RetentionPeriod int
	Retry           int
}

func NewStore(addr, org, bucket, token string, batchSize int, logger Logger) *Store {
	return &Store{
		client: &Client{
			addr:       addr,
			token:      token,
			httpClient: http.Client{Timeout: 4 * time.Second},
		},
		org:       org,
		bucket:    bucket,
		batchSize: batchSize,
		logger:    logger,
	}
}

func (s *Store) NeedSetup(ctx context.Context) (bool, error) {
	if s.client.token != "" {
		return false, nil
	}
	need, err := s.client.NeedSetup(ctx)
	if err != nil {
		return false, fmt.Errorf("could not check setup requirements: %w", err)
	}
	return need, nil
}

type Status struct {
	SetupRequired bool
	Ready         bool
	Healthy       bool
}

func (s *Store) GetStatus(ctx context.Context) (Status, error) {
	needSetup, err := s.NeedSetup(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("setup check failed: %w", err)
	}
	status := Status{
		SetupRequired: needSetup,
	}
	if status.SetupRequired {
		return status, nil
	}
	err = s.client.Ready(ctx)
	if err == nil {
		status.Ready = true
	}
	err = s.client.Health(ctx)
	if err == nil {
		status.Healthy = true
	}
	return status, nil
}

func (s *Store) Setup(ctx context.Context, username, password string, retentionPeriod time.Duration, tokenLocation string, fs afero.Fs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	token, err := s.client.Setup(username, password, s.org, s.bucket, retentionPeriod)
	if err != nil {
		return fmt.Errorf("could not setup influx database: %w", err)
	}
	s.client.token = token
	err = SaveToken(tokenLocation, token, fs)
	if err != nil {
		return fmt.Errorf("could not save influx token [%s]: %w", token, err)
	}
	s.logger.Infof("influx token saved to %s", tokenLocation)
	return nil
}

func (s *Store) Close() {
	s.client.SignOut()
}

func (s *Store) SaveMeasurement(ctx context.Context, measurement string, fields map[string]interface{}, tags map[string]string) error {
	return s.client.WriteMeasurement(ctx, s.org, s.bucket, measurement, fields, tags, time.Now(), s.logger)
}

func ReadToken(tokenLocation string, fs afero.Fs) (string, error) {
	tok, err := afero.ReadFile(fs, tokenLocation)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("could not read token: %w", err)
	}
	return strings.TrimSpace(string(tok)), nil
}

func SaveToken(tokenLocation, token string, fs afero.Fs) error {
	file, err := fs.OpenFile(tokenLocation, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not open token file: %w", err)
	}
	defer func() { _ = file.Close() }()
	_, err = file.Write([]byte(token))
	if err != nil {
		return fmt.Errorf("could not write token file: %w", err)
	}
	return nil
}
