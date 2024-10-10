package influx

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mklimuk/gockpit/metrics"

	"github.com/stretchr/testify/suite"

	"github.com/spf13/afero"

	"github.com/docker/go-connections/nat"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Suite struct {
	suite.Suite
	cli         *client.Client
	wg          sync.WaitGroup
	ret         chan error
	containerID string
	ctx         context.Context
	cancel      context.CancelFunc
	token       string
}

func (s *Suite) SetupSuite() {
	s.ret = make(chan error)
	var err error
	s.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		s.FailNowf("could not init docker client", "error: %v", err)
		return
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	reader, err := s.cli.ImagePull(s.ctx, "quay.io/influxdb/influxdb:v2.0.4", types.ImagePullOptions{})
	if err != nil {
		s.FailNow("could not pull influx image", "error: %v", err)
		return
	}
	_, _ = io.Copy(os.Stderr, reader)

	resp, err := s.cli.ContainerCreate(s.ctx, &container.Config{
		Image: "quay.io/influxdb/influxdb:v2.0.4",
	}, &container.HostConfig{PortBindings: nat.PortMap{
		"8086/tcp": []nat.PortBinding{
			{HostPort: "8086"},
		},
	}}, nil, nil, "influx_test")
	if err != nil {
		s.FailNow("could not create container", "error: %v", err)
		return
	}

	err = s.cli.ContainerStart(s.ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		s.FailNow("could not start container", "error: %v", err)
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		statusCh, errCh := s.cli.ContainerWait(s.ctx, resp.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			s.ret <- err
		case status := <-statusCh:
			if status.StatusCode != 0 {
				s.ret <- fmt.Errorf("invalid exit code: %d", status.StatusCode)
			}
			s.ret <- nil
		}
	}()

	s.containerID = resp.ID
	s.testSetup(s.T())
}

func (s *Suite) TearDownSuite() {
	err := s.cli.ContainerStop(s.ctx, s.containerID, container.StopOptions{})
	err = <-s.ret
	s.Assert().NoError(err)
	s.cancel()
	s.wg.Wait()
	err = s.cli.ContainerRemove(context.Background(), s.containerID, types.ContainerRemoveOptions{})
	s.Assert().NoError(err)
}

func (s *Suite) testSetup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tokenPath := "/tmp/token"
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.Mkdir("/tmp", 0666))
	store := NewStore("http://localhost:8086", "satsys", "hae", "", 32)
	// check there is no token
	token, err := ReadToken(tokenPath, fs)
	require.NoError(t, err)
	assert.Empty(t, token)
	// check if setup is needed
	need, err := store.NeedSetup(ctx)
	assert.NoError(t, err)
	assert.True(t, need)
	// perform setup
	err = store.Setup(ctx, "husar", "husar1234", 2*time.Hour, tokenPath, fs)
	assert.NoError(t, err)
	// setup is no more needed
	need, err = store.NeedSetup(ctx)
	assert.NoError(t, err)
	assert.False(t, need)
	// token is saved
	token, err = ReadToken(tokenPath, fs)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	s.token = token
	// check status
	status, err := store.GetStatus(ctx)
	assert.NoError(t, err)
	assert.False(t, status.SetupRequired)
	assert.True(t, status.Healthy)
	assert.True(t, status.Ready)
}

func (s *Suite) TestWrite() {
	store := NewStore("http://localhost:8086", "satsys", "hae", s.token, 32)
	require.NoError(s.T(), store.Publish(context.Background(), metrics.Metrics{
		Namespace: "test",
		Event:     "measure",
		Fields: map[string]interface{}{
			"string": "string",
			"int":    1,
			"float":  1.23,
		},
		Tags: map[string]string{"tag1": "t1", "tag2": "t2"},
	}))
}

func TestSuite(t *testing.T) {
	suite.Run(t, &Suite{})
}
