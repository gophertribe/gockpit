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
			w := WrappedInterfaceSettings{
				IfaceName: i.IfaceName,
				Mode:      int(i.Mode),
				Metric:    i.Metric,
			}
			if i.IfaceName == mainIf {
				w.Lan = true
			}
			if len(i.Addr.IP) > 0 {
				w.Addr = i.Addr.String()
			}
			if len(i.Gateway) > 0 {
				w.Gateway = i.Gateway.String()
			}
			res = append(res, w)
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
		ifaces := make([]InterfaceSettings, len(req))
		for i, r := range req {
			ifaces[i] = InterfaceSettings{
				IfaceName: r.IfaceName,
				Mode:      IPConfig(r.Mode),
				Metric:    r.Metric,
				Gateway:   net.ParseIP(r.Gateway),
			}
			if r.Addr != "" {
				ip, addr, err := net.ParseCIDR(r.Addr)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = enc.Encode(map[string]string{
						"error": err.Error(),
					})
					return
				}
				ifaces[i].Addr = net.IPNet{
					IP:   ip,
					Mask: addr.Mask,
				}
			}
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
