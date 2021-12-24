package gockpit

type Logger interface {
	Debug(string)
	Debugf(string, ...interface{})
	Info(string)
	Infof(string, ...interface{})
	Error(string)
	Errorf(string, ...interface{})
}
