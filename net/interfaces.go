package net

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/afero"
)

type IPConfig int

const (
	IPConfigStatic IPConfig = iota + 1
	IPConfigDHCP
)

type InterfaceSettings struct {
	IfaceName string
	Mode      IPConfig
	Addr      net.IPNet
	Gateway   net.IP
	Metric    int
}

type WrappedInterfaceSettings struct {
	IfaceName string `json:"name"`
	Mode      int    `json:"mode"`
	Addr      string `json:"addr,omitempty"`
	Gateway   string `json:"gw,omitempty"`
	Metric    int    `json:"metric"`
	Lan       bool   `json:"lan"`
}

func (i InterfaceSettings) IsStatic() bool {
	return i.Mode == IPConfigStatic
}

func WriteSettings(settings []InterfaceSettings, w io.Writer) error {
	err := interfacesTpl.Execute(w, settings)
	if err != nil {
		return fmt.Errorf("could not execute template: %w", err)
	}
	return nil
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

func ReadSettings(file io.Reader) ([]InterfaceSettings, error) {
	var res []InterfaceSettings
	var current *InterfaceSettings
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		// continue on empty lines
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "iface":
			if current != nil {
				res = append(res, *current)
			}
			if len(parts) < 4 {
				continue
			}
			switch parts[3] {
			case "static":
				current = &InterfaceSettings{IfaceName: parts[1], Mode: IPConfigStatic}
			case "dhcp":
				current = &InterfaceSettings{IfaceName: parts[1], Mode: IPConfigDHCP}
			}
		case "address":
			if current == nil {
				continue
			}
			if len(parts) < 2 {
				continue
			}
			ip, network, err := net.ParseCIDR(parts[1])
			var addr net.IPNet
			if err == nil {
				addr.IP = ip
				addr.Mask = network.Mask
			} else {
				addr.IP = net.ParseIP(parts[1])
				addr.Mask = addr.IP.DefaultMask()
			}
			current.Addr = addr
		case "gateway":
			if current == nil {
				continue
			}
			if len(parts) < 2 {
				continue
			}
			current.Gateway = net.ParseIP(parts[1])
		case "netmask":
			if current == nil {
				continue
			}
			if len(parts) < 2 {
				continue
			}
			current.Addr.Mask = net.IPMask(net.ParseIP(parts[1]))
		case "metric":
			if current == nil {
				continue
			}
			if len(parts) < 2 {
				continue
			}
			metric, err := strconv.Atoi(parts[1])
			if err != nil {
				return res, fmt.Errorf("could not parse metric value from %s: %w", parts[1], err)
			}
			current.Metric = metric
		default:
			continue
		}
	}
	if current != nil {
		res = append(res, *current)
	}
	if err := scanner.Err(); err != nil {
		return res, fmt.Errorf("could not scan input file: %w", err)
	}
	return res, nil
}

var interfacesTpl = template.Must(template.New("network/interfaces").Parse(networkInterfaces))

const networkInterfaces = `
source /etc/network/interfaces.d/*

# The loopback network interface
auto lo
iface lo inet loopback

{{ range . }}
auto {{ .IfaceName }}
{{ if .IsStatic }}iface {{ .IfaceName }} inet static
address {{ .Addr }}
{{ with .Gateway }}gateway {{ . }}{{end}}
{{- else }}iface {{ .IfaceName }} inet dhcp{{ end }}
metric {{ .Metric }}
{{ end }}
`
