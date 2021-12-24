package log

type Level int

var levels = map[Level]string{
	LevelDebug: "debug",
	LevelInfo:  "info",
	LevelError: "error",
}

func (l Level) String() string {
	return levels[l]
}

const (
	LevelDebug Level = 1 << iota
	LevelInfo
	LevelError
)
