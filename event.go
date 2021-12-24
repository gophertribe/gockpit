package gockpit

type Event struct {
	Namespace string      `json:"namespace"`
	Event     string      `json:"event"`
	Payload   interface{} `json:"payload"`
}
