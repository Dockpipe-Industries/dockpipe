//go:build linux

package dsmproxy

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"dockpipe/workflows/internal/synology/dsmauth"
	"dockpipe/workflows/internal/synology/hostidentity"
)

func singleProxyHeader(request *http.Request, name string) (string, error) {
	values := request.Header.Values(name)
	if len(values) != 1 || values[0] == "" {
		return "", errors.New("missing or ambiguous proxy metadata")
	}
	return values[0], nil
}

func Environment(request *http.Request, scriptName string) ([]string, error) {
	peer, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil || net.ParseIP(peer) == nil || !net.ParseIP(peer).IsLoopback() {
		return nil, errors.New("proxy peer must be loopback")
	}
	remoteAddress, err := singleProxyHeader(request, "X-Dockpipe-Remote-Addr")
	if err != nil || net.ParseIP(remoteAddress) == nil {
		return nil, errors.New("invalid forwarded remote address")
	}
	serverAddress, err := singleProxyHeader(request, "X-Dockpipe-Server-Addr")
	if err != nil || net.ParseIP(serverAddress) == nil {
		return nil, errors.New("invalid forwarded server address")
	}
	serverPort, err := singleProxyHeader(request, "X-Dockpipe-Server-Port")
	if err != nil {
		return nil, err
	}
	portNumber, err := strconv.ParseUint(serverPort, 10, 16)
	if err != nil || portNumber == 0 || strconv.FormatUint(portNumber, 10) != serverPort {
		return nil, errors.New("invalid forwarded server port")
	}
	scheme, err := singleProxyHeader(request, "X-Dockpipe-Scheme")
	if err != nil || scheme != "https" {
		return nil, errors.New("forwarded HTTPS is required")
	}
	parsedHost, err := url.Parse("https://" + request.Host)
	if err != nil || parsedHost.Host != request.Host || parsedHost.User != nil || parsedHost.Path != "" || parsedHost.RawQuery != "" || parsedHost.Fragment != "" || !hostidentity.ValidHostname(parsedHost.Hostname()) {
		return nil, errors.New("invalid forwarded HTTP host")
	}
	if strings.HasSuffix(request.Host, ":") {
		return nil, errors.New("invalid forwarded HTTP host port")
	}
	if hostPort := parsedHost.Port(); hostPort != "" {
		port, err := strconv.ParseUint(hostPort, 10, 16)
		if err != nil || port == 0 {
			return nil, errors.New("invalid forwarded HTTP host port")
		}
	}
	if origin := request.Header.Values("Origin"); len(origin) > 1 || (len(origin) == 1 && origin[0] != "https://"+request.Host) {
		return nil, errors.New("request origin does not match forwarded HTTPS host")
	}
	requestURI := scriptName
	if request.URL.RawQuery != "" {
		requestURI += "?" + request.URL.RawQuery
	}
	return dsmauth.Environment([]string{
		"GATEWAY_INTERFACE=CGI/1.1",
		"HTTP_COOKIE=" + request.Header.Get("Cookie"),
		"HTTP_HOST=" + request.Host,
		"HTTP_USER_AGENT=" + request.Header.Get("User-Agent"),
		"HTTP_X_SYNO_TOKEN=" + request.Header.Get("X-Syno-Token"),
		"REMOTE_ADDR=" + remoteAddress,
		"SERVER_ADDR=" + serverAddress,
		"SERVER_PORT=" + serverPort,
		"SERVER_NAME=" + parsedHost.Hostname(),
		"SERVER_PROTOCOL=" + request.Proto,
		"REQUEST_METHOD=GET",
		"REQUEST_URI=" + requestURI,
		"SCRIPT_NAME=" + scriptName,
		"QUERY_STRING=" + request.URL.RawQuery,
		"HTTPS=on",
	})
}
