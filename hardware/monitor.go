package hardware

import (
	"context"
	"sync"
	"time"

	"github.com/mklimuk/gockpit/state"

	"github.com/mklimuk/gockpit"

	"github.com/mklimuk/gockpit/metrics"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
)

type Metrics struct {
	Namespace string `json:"namespace"`
}

type MetricsStore interface {
	Publish(context.Context, metrics.Metrics) error
}

type Publisher interface {
	Publish(context.Context, interface{}) error
}

type Logger interface {
	Info(string)
}

type Monitor struct {
	mx     sync.Mutex
	state  map[string]interface{}
	logger Logger
}

func NewMonitor(logger gockpit.Logger) *Monitor {
	return &Monitor{
		state:  make(map[string]interface{}),
		logger: logger,
	}
}

func (hw *Monitor) Watch(ctx context.Context, mtr MetricsStore, pub Publisher, errs state.ErrorCollector, logger gockpit.Logger, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		wg.Done()
		logger.Info("starting hardware monitor watch routine")
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				hw.updateState(ctx, mtr, pub, errs, logger)
			case <-ctx.Done():
				hw.logger.Info("stopping hardware monitor watch routine")
				return
			}
		}
	}()
}

func (hw *Monitor) updateState(ctx context.Context, mtr MetricsStore, pub Publisher, errs state.ErrorCollector, logger gockpit.Logger) {
	hw.mx.Lock()
	defer hw.mx.Unlock()
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	memory, err := mem.VirtualMemoryWithContext(ctx)
	_ = errs.Collect(ctx, "hw", "mem_read", "could not read memory stats", err, state.Clearable)
	hw.state["mem_total"] = memory.Total
	hw.state["mem_used"] = memory.Used
	hw.state["mem_percent"] = memory.UsedPercent
	processor, err := cpu.PercentWithContext(ctx, 0, false)
	_ = errs.Collect(ctx, "hw", "cpu_read", "could not read cpu stats", err, state.Clearable)
	hw.state["cpu_percent"] = processor[0]
	diskUsage, err := disk.UsageWithContext(ctx, "/")
	_ = errs.Collect(ctx, "hw", "disk_read", "could not read disk usage stats", err, state.Clearable)
	hw.state["disk_percent"] = diskUsage.UsedPercent
	info, err := host.InfoWithContext(ctx)
	_ = errs.Collect(ctx, "hw", "host_read", "could not read hardware info", err, state.Clearable)
	hw.state["uptime"] = info.Uptime
	hw.state["boottime"] = info.BootTime
	if mtr != nil {
		err = mtr.Publish(ctx, metrics.New("hw", hw.state, nil))
		_ = errs.Collect(ctx, "hw", "metrics", "error publishing metrics", err, state.Clearable)
	}
	err = pub.Publish(ctx, gockpit.Event{
		Namespace: "hw",
		Event:     "metrics",
		Payload:   hw.state,
	})
	if err != nil {
		logger.Errorf("could not publish hardware metrics: %w", err)
	}
}
