package hardware

import (
	"context"
	"fmt"
	"github.com/nakabonne/tstorage"
	"sync"
	"time"

	"github.com/mklimuk/gockpit/state"

	"github.com/mklimuk/gockpit"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
)

var _ MetricsProvider = &Monitor{}

type Metrics struct {
	Namespace string `json:"namespace"`
}

type Publisher interface {
	Publish(context.Context, interface{}) error
}

type Logger interface {
	Info(string)
}

type Monitor struct {
	mx      sync.Mutex
	state   map[string]interface{}
	logger  Logger
	metrics tstorage.Storage
}

func NewMonitor(metricsPath string, retention time.Duration, logger gockpit.Logger) (*Monitor, error) {
	metrics, err := tstorage.NewStorage(tstorage.WithDataPath(metricsPath), tstorage.WithTimestampPrecision(tstorage.Milliseconds), tstorage.WithRetention(retention))
	if err != nil {
		return nil, fmt.Errorf("could not create hardware metrics storage: %w", err)
	}
	return &Monitor{
		state:   make(map[string]interface{}),
		logger:  logger,
		metrics: metrics,
	}, nil
}

var metricNames = []string{"mem_total", "mem_used", "mem_percent", "cpu_percent", "disk_percent", "uptime", "boottime"}

func (hw *Monitor) GetMetrics(from, to int64) (map[string][]*tstorage.DataPoint, error) {
	res := map[string][]*tstorage.DataPoint{}
	for _, m := range metricNames {
		points, err := hw.GetMetric(m, nil, from, to)
		if err != nil {
			return nil, fmt.Errorf("could not get metric %s: %w", m, err)
		}
		res[m] = points
	}
	return res, nil
}

func (hw *Monitor) GetMetric(metric string, labels []tstorage.Label, from, to int64) ([]*tstorage.DataPoint, error) {
	points, err := hw.metrics.Select(metric, labels, from, to)
	if err != nil {
		return nil, fmt.Errorf("could not select metric: %w", err)
	}
	return points, nil
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
	now := time.Now().UnixMilli()
	err = hw.metrics.InsertRows(
		[]tstorage.Row{
			{Metric: "mem_total", DataPoint: tstorage.DataPoint{Timestamp: now, Value: float64(memory.Total)}},
			{Metric: "mem_used", DataPoint: tstorage.DataPoint{Timestamp: now, Value: float64(memory.Used)}},
			{Metric: "mem_percent", DataPoint: tstorage.DataPoint{Timestamp: now, Value: memory.UsedPercent}},
			{Metric: "cpu_percent", DataPoint: tstorage.DataPoint{Timestamp: now, Value: processor[0]}},
			{Metric: "disk_percent", DataPoint: tstorage.DataPoint{Timestamp: now, Value: diskUsage.UsedPercent}},
			{Metric: "uptime", DataPoint: tstorage.DataPoint{Timestamp: now, Value: float64(info.Uptime)}},
			{Metric: "boottime", DataPoint: tstorage.DataPoint{Timestamp: now, Value: float64(info.BootTime)}},
		})
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
