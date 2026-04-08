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
		{
			IfaceName: "eth0",
			Mode:      IPConfigStatic,
			Addrs: []InterfaceAddress{{
				Addr: net.IPNet{
					IP:   net.ParseIP("192.168.13.130"),
					Mask: net.IPMask{255, 255, 255, 0},
				},
				Gateway: net.ParseIP("192.168.13.1"),
			}},
			Metric: 900,
		},
		{
			IfaceName: "eth1",
			Mode:      IPConfigDHCP,
			Metric:    100,
		},
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

func TestWriteMultiAddress(t *testing.T) {
	set := []InterfaceSettings{
		{
			IfaceName: "enp1s0",
			Mode:      IPConfigStatic,
			Addrs: []InterfaceAddress{
				{
					Addr:    net.IPNet{IP: net.ParseIP("192.168.13.80"), Mask: net.CIDRMask(24, 32)},
					Gateway: net.ParseIP("192.168.13.1"),
				},
				{
					Addr:    net.IPNet{IP: net.ParseIP("192.168.13.81"), Mask: net.CIDRMask(24, 32)},
					Gateway: net.ParseIP("192.168.13.1"),
				},
			},
		},
		{
			IfaceName: "enp2s0",
			Mode:      IPConfigStatic,
			Addrs: []InterfaceAddress{{
				Addr: net.IPNet{IP: net.ParseIP("10.172.0.1"), Mask: net.CIDRMask(24, 32)},
			}},
			Metric: 300,
		},
	}
	var buf bytes.Buffer
	require.NoError(t, WriteSettings(set, &buf))
	assert.Equal(t, `
source /etc/network/interfaces.d/*

# The loopback network interface
auto lo
iface lo inet loopback


auto enp1s0
iface enp1s0 inet static
address 192.168.13.80/24
gateway 192.168.13.1
metric 0

iface enp1s0 inet static
address 192.168.13.81/24
gateway 192.168.13.1
metric 0


auto enp2s0
iface enp2s0 inet static
address 10.172.0.1/24
metric 300

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
		if assert.Len(t, s[0].Addrs, 1) {
			assert.Equal(t, net.IPv4(192, 168, 13, 130), s[0].Addrs[0].Addr.IP)
			assert.Equal(t, net.IPv4Mask(255, 255, 255, 0), s[0].Addrs[0].Addr.Mask)
			assert.Equal(t, net.IPv4(192, 168, 13, 1), s[0].Addrs[0].Gateway)
		}
		assert.Equal(t, "eth1", s[1].IfaceName)
		assert.Equal(t, IPConfigDHCP, s[1].Mode)
		assert.Equal(t, 100, s[1].Metric)
	}
}

func TestParserMultiAddress(t *testing.T) {
	s, err := ReadSettings(bytes.NewBufferString(`source /etc/network/interfaces.d/*

# The loopback network interface
auto lo
iface lo inet loopback

allow-hotplug enp1s0
iface enp1s0 inet static
    address 192.168.13.80/24
    gateway 192.168.13.1

iface enp1s0 inet static
    address 192.168.13.81/24
    gateway 192.168.13.1

allow-hotplug enp2s0
iface enp2s0 inet static
address 10.172.0.1
netmask 255.255.255.0
metric 300

allow-hotplug enp3s0
iface enp3s0 inet static
address 10.172.0.1
netmask 255.255.255.0
metric 200
`))
	require.NoError(t, err)
	if assert.Len(t, s, 3) {
		assert.Equal(t, "enp1s0", s[0].IfaceName)
		assert.Equal(t, IPConfigStatic, s[0].Mode)
		if assert.Len(t, s[0].Addrs, 2) {
			assert.Equal(t, net.IPv4(192, 168, 13, 80), s[0].Addrs[0].Addr.IP)
			assert.Equal(t, net.IPv4Mask(255, 255, 255, 0), s[0].Addrs[0].Addr.Mask)
			assert.Equal(t, net.IPv4(192, 168, 13, 1), s[0].Addrs[0].Gateway)
			assert.Equal(t, net.IPv4(192, 168, 13, 81), s[0].Addrs[1].Addr.IP)
			assert.Equal(t, net.IPv4Mask(255, 255, 255, 0), s[0].Addrs[1].Addr.Mask)
			assert.Equal(t, net.IPv4(192, 168, 13, 1), s[0].Addrs[1].Gateway)
		}

		assert.Equal(t, "enp2s0", s[1].IfaceName)
		assert.Equal(t, 300, s[1].Metric)
		if assert.Len(t, s[1].Addrs, 1) {
			assert.Equal(t, net.IPv4(10, 172, 0, 1), s[1].Addrs[0].Addr.IP)
		}

		assert.Equal(t, "enp3s0", s[2].IfaceName)
		assert.Equal(t, 200, s[2].Metric)
	}
}

func TestEmpty(t *testing.T) {
	var addr net.IPNet
	assert.True(t, len(addr.IP) == 0)
}
