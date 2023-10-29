package schema

import (
	"fmt"
	"net"
	"strconv"
)

const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"
)

func prefixer(prefix, flagName string) string {
	if prefix == "" {
		return flagName
	}
	return prefix + "-" + flagName
}

func SplitHostPort(addr string) (host string, port int, err error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", -1, err
	}

	if portStr == "" {
		return "", -1, fmt.Errorf("missing port in address: %s", addr)
	}

	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", -1, err
	}

	return host, port, nil
}


