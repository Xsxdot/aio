package utils

import (
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

func GetPublicIP() string {
	resp, err := http.Get("https://ifconfig.me/ip")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(ip))
}

func GetLocalIP() string {
	loop := "localhost"
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return loop
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return loop
}
