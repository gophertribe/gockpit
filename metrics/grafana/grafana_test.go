package grafana

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrafana(t *testing.T) {
	g := New("http://192.168.88.199:3000")
	require.NoError(t, g.SetupInfluxDatasource(context.TODO(), "test", "test", "test", "test", "test", nil))
}
