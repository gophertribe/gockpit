package grafana

import (
	"testing"
)

func TestGrafana(t *testing.T) {
	_ = New("http://192.168.88.199:3000")
}
