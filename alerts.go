package gockpit

import "time"

type AlertStrategy func(*Alert) bool

var AlertStrategyClear AlertStrategy = func(*Alert) bool { return true }
var AlertStrategyLatch AlertStrategy = func(*Alert) bool { return false }

type Alert struct {
	IsSet          bool      `json:"isSet"`
	FirstOccurence time.Time `json:"firstOccurrence"`
	LastOccurrence time.Time `json:"lastOccurrence"`
	update         func(interface{}, *Alert)
}

func (a *Alert) Clear() {
	a.IsSet = false
}

type Alerts map[string]*Alert

func NewBoolAlert(strategy AlertStrategy) *Alert {
	return &Alert{
		update: func(i interface{}, a *Alert) {
			b, ok := i.(bool)
			if !ok {
				return
			}
			if b {
				a.IsSet = true
				return
			}
			if strategy(a) {
				a.IsSet = false
			}
		},
	}
}

func NewInverseBoolAlert(strategy AlertStrategy) *Alert {
	return &Alert{
		update: func(i interface{}, a *Alert) {
			b, ok := i.(bool)
			if !ok {
				return
			}
			if !b {
				a.IsSet = true
				return
			}
			if strategy(a) {
				a.IsSet = false
			}
		},
	}
}

func NewMaxFloatAlert(max float64, strategy AlertStrategy) *Alert {
	return &Alert{
		update: func(i interface{}, a *Alert) {
			switch val := i.(type) {
			case float32:
				if float64(val) >= max {
					a.IsSet = true
					return
				}
			case float64:
				if val >= max {
					a.IsSet = true
				}
			default:
				return
			}
			if strategy(a) {
				a.IsSet = false
			}
		},
	}
}
