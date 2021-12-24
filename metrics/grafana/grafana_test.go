package grafana

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrafana(t *testing.T) {
	g := New("http://192.168.88.199:3000", []string{})
	require.NoError(t, g.SetupInflux())
}
