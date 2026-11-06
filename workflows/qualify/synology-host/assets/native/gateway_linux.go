//go:build linux

package main

import (
	"net/http"
	"strings"

	"dockpipe/workflows/internal/synology/dsmproxy"
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

func proxiedAuthenticationEnvironment(request *http.Request) ([]string, error) {
	return dsmproxy.Environment(request, reportScriptName)
}
