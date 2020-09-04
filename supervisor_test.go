package gockpit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSupervisor_Run(t *testing.T) {
	sup := NewSupervisor("test", WithSamplingInterval(20*time.Millisecond))
	var expectedCurrent State
	sup.AddListener(func(current *State) {
		assert.Equal(t, expectedCurrent, current, "current state mismatch")
	})
	var p probeMock
	sup.AddProbe("p1", 15*time.Millisecond, ProbeFunc(p.UpdateState))
	sup.Run(context.Background())
	p.On("Read").Return(10, nil).Once()
	expectedCurrent = State{}
	time.Sleep(25 * time.Millisecond)
	p.On("Read").Return(11, nil).Once()
	expectedCurrent = State{data: map[string]interface{}{"_errors": Errors{}, "p1": 10}}
	time.Sleep(20 * time.Millisecond)
	p.On("Read").Return(12, nil).Once()
	expectedCurrent = State{data: map[string]interface{}{"_errors": Errors{}, "p1": 11}}
	time.Sleep(20 * time.Millisecond)
	p.On("Read").Return(0, fmt.Errorf("dummy")).Once()
	expectedCurrent = State{data: map[string]interface{}{"_errors": Errors{}, "p1": 12}}
	time.Sleep(20 * time.Millisecond)
	sup.Stop()
}

type probeMock struct {
	mock.Mock
}

func (m *probeMock) UpdateState(ctx context.Context, state *StateMutation) {

}

func (m *probeMock) SetupState(ctx context.Context, state *State) {

}
