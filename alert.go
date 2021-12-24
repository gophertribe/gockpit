package gockpit

type AlertDispatcher interface {
	Dispatch(name string, alert interface{})
}
