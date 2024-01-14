package hardware

import (
	"context"
	"sync"
	"time"

	"github.com/mklimuk/gockpit/state"

	"github.com/mklimuk/gockpit"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
)

type Metrics struct {
	Namespace string `json:"namespace"`
}

type Publisher interface {
	Publish(context.Context, interface{}) error
}

type Logger interface {
	Info(string)
}

type MetricsWriter interface {
	HardwareStateUpdate(s State) error
}

type State struct {
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	CpuPercent  float64 `json:"cpu_percent"`
	DiskPercent float64 `json:"disk_percent"`
	Uptime      uint64  `json:"uptime"`
	Boottime    uint64  `json:"boottime"`
}

type Monitor struct {
	mx      sync.Mutex
	state   State
	logger  Logger
	metrics MetricsWriter
}

func NewMonitor(mw MetricsWriter, logger gockpit.Logger) (*Monitor, error) {
	return &Monitor{
		logger:  logger,
		metrics: mw,
	}, nil
}

func (hw *Monitor) Watch(ctx context.Context, pub Publisher, errs state.ErrorCollector, logger gockpit.Logger, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		wg.Done()
		logger.Info("starting hardware monitor watch routine")
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				hw.updateState(ctx, pub, errs, logger)
			case <-ctx.Done():
				hw.logger.Info("stopping hardware monitor watch routine")
				return
			}
		}
	}()
}

func (hw *Monitor) updateState(ctx context.Context, pub Publisher, errs state.ErrorCollector, logger gockpit.Logger) {
	hw.mx.Lock()
	defer hw.mx.Unlock()
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	memory, err := mem.VirtualMemoryWithContext(ctx)
	_ = errs.Collect(ctx, "hw", "mem_read", "could not read memory stats", err, state.Clearable)
	hw.state.MemTotal = memory.Total
	hw.state.MemUsed = memory.Used
	hw.state.MemPercent = memory.UsedPercent
	processor, err := cpu.PercentWithContext(ctx, 0, false)
	_ = errs.Collect(ctx, "hw", "cpu_read", "could not read cpu stats", err, state.Clearable)
	hw.state.CpuPercent = processor[0]
	diskUsage, err := disk.UsageWithContext(ctx, "/")
	_ = errs.Collect(ctx, "hw", "disk_read", "could not read disk usage stats", err, state.Clearable)
	hw.state.DiskPercent = diskUsage.UsedPercent
	info, err := host.InfoWithContext(ctx)
	_ = errs.Collect(ctx, "hw", "host_read", "could not read hardware info", err, state.Clearable)
	hw.state.Uptime = info.Uptime
	hw.state.Boottime = info.BootTime
	err = hw.metrics.HardwareStateUpdate(hw.state)
	_ = errs.Collect(ctx, "hw", "metrics", "error publishing metrics", err, state.Clearable)
	err = pub.Publish(ctx, gockpit.Event{
		Namespace: "hw",
		Event:     "metrics",
		Payload:   hw.state,
	})
	if err != nil {
		logger.Errorf("could not publish hardware metrics: %w", err)
	}
}
