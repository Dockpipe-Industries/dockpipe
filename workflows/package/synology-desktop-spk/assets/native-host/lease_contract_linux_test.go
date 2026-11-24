//go:build linux

package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestNativeRenewalReportsTheActualBrowserLeaseDeadline(t *testing.T) {
	server, _ := fixtureServer(t)
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	server.leases.now = func() time.Time { return now }
	request := fixtureRequest(http.MethodPost, `{"action":"prepare"}`)
	response, report := requestReport(t, server, request)
	if response.Code != http.StatusOK || report.LeaseExpiresAt == nil || !report.LeaseExpiresAt.Equal(now.Add(leaseLifetime)) {
		t.Fatal("prepare omitted the lease deadline required by the DSM launcher")
	}
	var browserReport map[string]json.RawMessage
	if json.Unmarshal(response.Body.Bytes(), &browserReport) != nil {
		t.Fatal("invalid browser report")
	}
	var serializedDeadline string
	if json.Unmarshal(browserReport["lease_expires_at"], &serializedDeadline) != nil {
		t.Fatal("browser lease expiry is not a timestamp string")
	}
	if deadline, err := time.Parse(time.RFC3339Nano, serializedDeadline); err != nil || !deadline.Equal(*report.LeaseExpiresAt) {
		t.Fatal("browser lease expiry does not match server authorization")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("prepare did not establish one browser lease")
	}
	attachCookie(request, cookies[0])
	token, allowed := server.leases.gate(request)
	if !allowed {
		t.Fatal("prepare report did not correspond to a live gate")
	}
	server.leases.mutex.Lock()
	actual := server.leases.browsers[token].expires
	server.leases.mutex.Unlock()
	if !actual.Equal(*report.LeaseExpiresAt) {
		t.Fatal("reported expiry differs from the stored lease")
	}

	now = now.Add(10 * time.Second)
	heartbeat := fixtureRequest(http.MethodPost, `{"action":"heartbeat"}`)
	attachCookie(heartbeat, cookies[0])
	response, report = requestReport(t, server, heartbeat)
	if response.Code != http.StatusOK || report.LeaseExpiresAt == nil || !report.LeaseExpiresAt.Equal(now.Add(leaseLifetime)) {
		t.Fatal("heartbeat did not return its renewed lease deadline")
	}
	now = *report.LeaseExpiresAt
	if server.leases.gateAlive(token) {
		t.Fatal("browser authority survived its reported expiry")
	}
}
