//go:build linux

package main

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const authenticationListenAddress = "127.0.0.1:38140"
const reportScriptName = "/webman/3rdparty/dockpipe-host-qualification/host-auth.cgi"

func authenticationGateway(auth authenticator, query serviceQuery) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secureHeaders(writer)
		if request.URL.Path != "/report" {
			writeFailure(writer, http.StatusNotFound, "not_found")
			return
		}
		if !readOnlyRequest(writer, request) {
			return
		}
		environment, err := proxiedAuthenticationEnvironment(request)
		if err != nil {
			writeFailure(writer, http.StatusForbidden, "invalid_proxy_metadata")
			return
		}
		if strings.TrimSpace(request.Header.Get("Cookie")) == "" {
			writeFailure(writer, http.StatusUnauthorized, "unauthenticated")
			return
		}
		authenticationHandler(environment, auth, query).ServeHTTP(writer, request)
	})
}

func singleProxyHeader(request *http.Request, name string) (string, error) {
	values := request.Header.Values(name)
	if len(values) != 1 || values[0] == "" {
		return "", errors.New("missing or ambiguous proxy metadata")
	}
	return values[0], nil
}

func proxiedAuthenticationEnvironment(request *http.Request) ([]string, error) {
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
	if err != nil || parsedHost.Host != request.Host || parsedHost.User != nil || parsedHost.Path != "" || parsedHost.RawQuery != "" || parsedHost.Fragment != "" || !validHostname(parsedHost.Hostname()) {
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
	requestURI := reportScriptName
	if request.URL.RawQuery != "" {
		requestURI += "?" + request.URL.RawQuery
	}
	return authenticationEnvironment([]string{
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
		"SCRIPT_NAME=" + reportScriptName,
		"QUERY_STRING=" + request.URL.RawQuery,
		"HTTPS=on",
	})
}
