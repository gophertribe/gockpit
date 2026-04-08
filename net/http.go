package net

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/spf13/afero"
)

func GetInterfacesHandler(fs afero.Fs, path string, mainIf string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ifaces, err := ReadSettingsFromFile(fs, path)
		enc := json.NewEncoder(w)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = enc.Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}
		var res []WrappedInterfaceSettings
		for _, i := range ifaces {
			ws := WrappedInterfaceSettings{
				IfaceName: i.IfaceName,
				Mode:      int(i.Mode),
				Metric:    i.Metric,
			}
			if i.IfaceName == mainIf {
				ws.Lan = true
			}
			for _, a := range i.Addrs {
				wa := WrappedAddress{}
				if len(a.Addr.IP) > 0 {
					wa.Addr = a.Addr.String()
				}
				if len(a.Gateway) > 0 {
					wa.Gateway = a.Gateway.String()
				}
				ws.Addrs = append(ws.Addrs, wa)
			}
			if len(i.Addrs) > 0 {
				if len(i.Addrs[0].Addr.IP) > 0 {
					ws.Addr = i.Addrs[0].Addr.String()
				}
				if len(i.Addrs[0].Gateway) > 0 {
					ws.Gateway = i.Addrs[0].Gateway.String()
				}
			}
			res = append(res, ws)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = enc.Encode(res)
	}
}

func InterfacesUpdateHandler(fs afero.Fs, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req []WrappedInterfaceSettings
		dec := json.NewDecoder(r.Body)
		defer func() { _ = r.Body.Close() }()
		err := dec.Decode(&req)
		enc := json.NewEncoder(w)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = enc.Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}
		var ifaces []InterfaceSettings
		for _, ri := range req {
			iface := InterfaceSettings{
				IfaceName: ri.IfaceName,
				Mode:      IPConfig(ri.Mode),
				Metric:    ri.Metric,
			}
			if len(ri.Addrs) > 0 {
				for _, a := range ri.Addrs {
					ia := InterfaceAddress{
						Gateway: net.ParseIP(a.Gateway),
					}
					if a.Addr != "" {
						ip, addr, err := net.ParseCIDR(a.Addr)
						if err != nil {
							w.WriteHeader(http.StatusBadRequest)
							_ = enc.Encode(map[string]string{
								"error": err.Error(),
							})
							return
						}
						ia.Addr = net.IPNet{IP: ip, Mask: addr.Mask}
					}
					iface.Addrs = append(iface.Addrs, ia)
				}
			} else if ri.Addr != "" {
				ip, addr, err := net.ParseCIDR(ri.Addr)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = enc.Encode(map[string]string{
						"error": err.Error(),
					})
					return
				}
				iface.Addrs = []InterfaceAddress{{
					Addr:    net.IPNet{IP: ip, Mask: addr.Mask},
					Gateway: net.ParseIP(ri.Gateway),
				}}
			}
			ifaces = append(ifaces, iface)
		}
		err = WriteSettingsToFile(ifaces, fs, path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = enc.Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
