package net

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

type IPConfig int

const (
	IPConfigStatic IPConfig = iota + 1
	IPConfigDHCP
)

type InterfaceAddress struct {
	Addr    net.IPNet
	Gateway net.IP
}

type InterfaceSettings struct {
	IfaceName string
	Mode      IPConfig
	Addrs     []InterfaceAddress
	Metric    int
}

type WrappedAddress struct {
	Addr    string `json:"addr,omitempty"`
	Gateway string `json:"gw,omitempty"`
}

type WrappedInterfaceSettings struct {
	IfaceName string           `json:"name"`
	Mode      int              `json:"mode"`
	Addr      string           `json:"addr,omitempty"`
	Gateway   string           `json:"gw,omitempty"`
	Addrs     []WrappedAddress `json:"addrs,omitempty"`
	Metric    int              `json:"metric"`
	Lan       bool             `json:"lan"`
}

func (i InterfaceSettings) IsStatic() bool {
	return i.Mode == IPConfigStatic
}

func (i InterfaceSettings) PrimaryAddr() net.IPNet {
	if len(i.Addrs) > 0 {
		return i.Addrs[0].Addr
	}
	return net.IPNet{}
}

func (i InterfaceSettings) PrimaryGateway() net.IP {
	if len(i.Addrs) > 0 {
		return i.Addrs[0].Gateway
	}
	return nil
}

func WriteSettings(settings []InterfaceSettings, w io.Writer) error {
	var buf strings.Builder
	buf.WriteString("\nsource /etc/network/interfaces.d/*\n\n# The loopback network interface\nauto lo\niface lo inet loopback\n")
	for _, iface := range settings {
		buf.WriteString(fmt.Sprintf("\n\nauto %s\n", iface.IfaceName))
		if iface.IsStatic() {
			for i, addr := range iface.Addrs {
				if i > 0 {
					buf.WriteString("\n")
				}
				buf.WriteString(fmt.Sprintf("iface %s inet static\naddress %s\n", iface.IfaceName, addr.Addr.String()))
				if len(addr.Gateway) > 0 {
					buf.WriteString(fmt.Sprintf("gateway %s\n", addr.Gateway.String()))
				}
				buf.WriteString(fmt.Sprintf("metric %d\n", iface.Metric))
			}
		} else {
			buf.WriteString(fmt.Sprintf("iface %s inet dhcp\nmetric %d\n", iface.IfaceName, iface.Metric))
		}
	}
	buf.WriteString("\n")
	_, err := io.WriteString(w, buf.String())
	return err
}

func WriteSettingsToFile(settings []InterfaceSettings, fs afero.Fs, path string) error {
	file, err := fs.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("could not open target file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	return WriteSettings(settings, file)
}

func ReadSettingsFromFile(fs afero.Fs, path string) ([]InterfaceSettings, error) {
	file, err := fs.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open source file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	return ReadSettings(file)
}

type parsedBlock struct {
	ifaceName string
	mode      IPConfig
	addr      net.IPNet
	gateway   net.IP
	metric    int
}

func ReadSettings(file io.Reader) ([]InterfaceSettings, error) {
	var blocks []parsedBlock
	var current *parsedBlock
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "iface":
			if current != nil {
				blocks = append(blocks, *current)
			}
			if len(parts) < 4 {
				current = nil
				continue
			}
			switch parts[3] {
			case "static":
				current = &parsedBlock{ifaceName: parts[1], mode: IPConfigStatic}
			case "dhcp":
				current = &parsedBlock{ifaceName: parts[1], mode: IPConfigDHCP}
			default:
				current = nil
			}
		case "address":
			if current == nil || len(parts) < 2 {
				continue
			}
			ip, network, err := net.ParseCIDR(parts[1])
			if err == nil {
				current.addr.IP = ip
				current.addr.Mask = network.Mask
			} else {
				current.addr.IP = net.ParseIP(parts[1])
				if current.addr.IP != nil {
					current.addr.Mask = current.addr.IP.DefaultMask()
				}
			}
		case "gateway":
			if current == nil || len(parts) < 2 {
				continue
			}
			current.gateway = net.ParseIP(parts[1])
		case "netmask":
			if current == nil || len(parts) < 2 {
				continue
			}
			maskIP := net.ParseIP(parts[1])
			if maskIP != nil {
				if v4 := maskIP.To4(); v4 != nil {
					current.addr.Mask = net.IPMask(v4)
				} else {
					current.addr.Mask = net.IPMask(maskIP)
				}
			}
		case "metric":
			if current == nil || len(parts) < 2 {
				continue
			}
			metric, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("could not parse metric value from %s: %w", parts[1], err)
			}
			current.metric = metric
		default:
			continue
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("could not scan input file: %w", err)
	}

	seen := make(map[string]int)
	var result []InterfaceSettings
	for _, b := range blocks {
		if idx, ok := seen[b.ifaceName]; ok {
			if len(b.addr.IP) > 0 {
				result[idx].Addrs = append(result[idx].Addrs, InterfaceAddress{
					Addr:    b.addr,
					Gateway: b.gateway,
				})
			}
			if b.metric > 0 && result[idx].Metric == 0 {
				result[idx].Metric = b.metric
			}
		} else {
			seen[b.ifaceName] = len(result)
			iface := InterfaceSettings{
				IfaceName: b.ifaceName,
				Mode:      b.mode,
				Metric:    b.metric,
			}
			if len(b.addr.IP) > 0 {
				iface.Addrs = []InterfaceAddress{{
					Addr:    b.addr,
					Gateway: b.gateway,
				}}
			}
			result = append(result, iface)
		}
	}
	return result, nil
}
