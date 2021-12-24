package net

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestWriteSettings(t *testing.T) {
	set := []InterfaceSettings{
		{IfaceName: "eth0",
			Mode: IPConfigStatic,
			Addr: net.IPNet{
				IP:   net.ParseIP("192.168.13.130"),
				Mask: net.IPMask{255, 255, 255, 0}},
			Gateway: net.ParseIP("192.168.13.1"),
			Metric:  900,
		},
		{IfaceName: "eth1",
			Mode:   IPConfigDHCP,
			Metric: 100},
	}
	var buf bytes.Buffer
	require.NoError(t, WriteSettings(set, &buf))
	assert.Equal(t, `
source /etc/network/interfaces.d/*

# The loopback network interface
auto lo
iface lo inet loopback


auto eth0
iface eth0 inet static
address 192.168.13.130/24
gateway 192.168.13.1
metric 900

auto eth1
iface eth1 inet dhcp
metric 100

`, buf.String())
}

func TestParser(t *testing.T) {
	s, err := ReadSettings(bytes.NewBufferString(`source /etc/network/interfaces.d/*

# The loopback network interface
auto lo
iface lo inet loopback


auto eth0
iface eth0 inet static
address 192.168.13.130/24
gateway 192.168.13.1
metric 900

auto eth1
iface eth1 inet dhcp
metric 100

`))
	require.NoError(t, err)
	if assert.Len(t, s, 2) {
		assert.Equal(t, "eth0", s[0].IfaceName)
		assert.Equal(t, IPConfigStatic, s[0].Mode)
		assert.Equal(t, 900, s[0].Metric)
		assert.Equal(t, net.IPv4(192, 168, 13, 130), s[0].Addr.IP)
		assert.Equal(t, net.IPv4Mask(255, 255, 255, 0), s[0].Addr.Mask)
		assert.Equal(t, net.IPv4(192, 168, 13, 1), s[0].Gateway)
		assert.Equal(t, "eth1", s[1].IfaceName)
		assert.Equal(t, IPConfigDHCP, s[1].Mode)
		assert.Equal(t, 100, s[1].Metric)
	}
}

func TestEmpty(t *testing.T) {
	var addr net.IPNet
	assert.True(t, len(addr.IP) == 0)
}
