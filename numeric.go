package gockpit

import "fmt"

var (
	ErrOffLimitMin = fmt.Errorf("gauge off min limit")
	ErrOffLimitMax = fmt.Errorf("gauge off max limit")
)

type Int struct {
	min int
	max int
	update func() (int, error)
}

func (g *Int) Read() (interface{},error) {
	next,err := g.update()
	if err != nil {
		return next, fmt.Errorf("could not update probe: %w", err)
	}
	if next < g.min {
		return next, ErrOffLimitMin
	}
	if next > g.max {
		return next, ErrOffLimitMax
	}
	return next, nil
}

type Float struct {
	min float32
	max float32
	update func() (float32, error)
}

func (g *Float) Read() (interface{},error) {
	next,err := g.update()
	if err != nil {
		return next, fmt.Errorf("could not update probe: %w", err)
	}
	if next < g.min {
		return next, ErrOffLimitMin
	}
	if next > g.max {
		return next, ErrOffLimitMax
	}
	return next, nil
}
