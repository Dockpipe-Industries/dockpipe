//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/nativehost"
	"dockpipe/workflows/internal/synology/localaccount"
)

type fixtureBroker struct {
	onExpect     func()
	status       nativehost.Status
	expectations int
	revocations  int
	authorize    func() (nativehost.Grant, error)
}

func (broker *fixtureBroker) ExpectWorker(uid, gid uint32, nonce string, _ time.Time) error {
	if uid != 1026 || gid != 100 || len(nonce) != 64 {
		return errors.New("unexpected worker authority")
	}
	if broker.authorize != nil {
		if _, err := broker.authorize(); err != nil {
			return err
		}
	}
	if broker.onExpect != nil {
		broker.onExpect()
	}
	broker.expectations++
	return nil
}
func (broker *fixtureBroker) Status() nativehost.Status { return broker.status }
func (broker *fixtureBroker) Revoke() error {
	broker.revocations++
	broker.status = nativehost.Status{}
	return nil
}

type deadlineRecorder struct{ *httptest.ResponseRecorder }

func (*deadlineRecorder) SetReadDeadline(time.Time) error  { return nil }
func (*deadlineRecorder) SetWriteDeadline(time.Time) error { return nil }

func fixtureServer(t *testing.T) (*nativeServer, *fixtureBroker) {
	broker := &fixtureBroker{}
	server := &nativeServer{
		leases: fixtureLeases(t), broker: broker,
		authenticate: func(context.Context, []string) (string, error) { return "operator", nil },
		resolve:      func(string) (localaccount.Account, error) { return fixtureAccount(), nil },
		desktop:      http.NotFoundHandler(),
	}
	broker.authorize = server.leases.authorize
	return server, broker
}

func requestReport(t *testing.T, server *nativeServer, request *http.Request) (*deadlineRecorder, publicReport) {
	t.Helper()
	recorder := &deadlineRecorder{httptest.NewRecorder()}
	server.ServeHTTP(recorder, request)
	var report publicReport
	_ = json.Unmarshal(recorder.Body.Bytes(), &report)
	return recorder, report
}

func TestStatusReadDoesNotGrantAndStarterFinishesWithoutRevoking(t *testing.T) {
	server, broker := fixtureServer(t)
	recorder, report := requestReport(t, server, fixtureRequest(http.MethodGet, ""))
	if recorder.Code != 200 || report.Challenge != nil || report.Result != nil || broker.expectations != 0 {
		t.Fatal("status read changed authority")
	}
	recorder, report = requestReport(t, server, fixtureRequest(http.MethodPost, `{"action":"prepare"}`))
	if recorder.Code != 200 || !report.Created || report.ExpectedUID != 1026 || broker.expectations != 1 || len(recorder.Result().Cookies()) != 1 {
		t.Fatal("verified prepare failed")
	}
	broker.status = nativehost.Status{Schema: nativehost.StatusSchema, Available: true, Connected: true, Node: "test-nas", Username: "operator", UID: 1026, GID: 100}
	nonce := report.Challenge.Nonce
	body, _ := json.Marshal(map[string]string{"action": "finish", "nonce": nonce})
	recorder, report = requestReport(t, server, fixtureRequest(http.MethodPost, string(body)))
	if recorder.Code != 200 || report.Challenge != nil || report.Result == nil || !report.Result.Available {
		t.Fatal("finishing starter erased native access proof")
	}
	recorder, _ = requestReport(t, server, fixtureRequest(http.MethodPost, `{"action":"disconnect"}`))
	if recorder.Code != 200 || broker.revocations != 1 {
		t.Fatal("explicit disable did not revoke worker")
	}
	recorder, _ = requestReport(t, server, fixtureRequest(http.MethodPost, `{"action":"heartbeat"}`))
	if recorder.Code != 409 {
		t.Fatal("heartbeat revived disabled worker")
	}
}

func TestNativeHTTPRejectsBadAuthorityAndPayloads(t *testing.T) {
	for _, scenario := range []string{"missing-token", "missing-cookie", "wrong-origin", "no-origin", "other-operator", "arbitrary-command", "oversized", "trailing-json", "unknown-action"} {
		t.Run(scenario, func(t *testing.T) {
			server, broker := fixtureServer(t)
			request := fixtureRequest(http.MethodPost, `{"action":"prepare"}`)
			switch scenario {
			case "missing-token":
				request.Header.Del("X-Syno-Token")
			case "missing-cookie":
				request.Header.Del("Cookie")
			case "wrong-origin":
				request.Header.Set("Origin", "https://other.example")
			case "no-origin":
				request.Header.Del("Origin")
			case "other-operator":
				_, _, _ = server.leases.prepare(fixtureAccount())
				server.resolve = func(string) (localaccount.Account, error) {
					return localaccount.Account{Username: "other", UID: 1027, GID: 100}, nil
				}
			case "arbitrary-command":
				request = fixtureRequest(http.MethodPost, `{"action":"prepare","command":"arbitrary"}`)
			case "oversized":
				request = fixtureRequest(http.MethodPost, strings.Repeat(" ", 1025))
			case "trailing-json":
				request = fixtureRequest(http.MethodPost, `{"action":"prepare"}{}`)
			case "unknown-action":
				request = fixtureRequest(http.MethodPost, `{"action":"execute"}`)
			}
			recorder, _ := requestReport(t, server, request)
			if recorder.Code < 400 || broker.expectations != 0 {
				t.Fatal("unverified request reserved host worker")
			}
		})
	}
}

func TestConcurrentPrepareWaitsForWorkerExpectationCommit(t *testing.T) {
	server, broker := fixtureServer(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	broker.onExpect = func() {
		close(entered)
		<-release
	}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		requestReport(t, server, fixtureRequest(http.MethodPost, `{"action":"prepare"}`))
	}()
	<-entered
	secondDone := make(chan publicReport, 1)
	go func() {
		_, report := requestReport(t, server, fixtureRequest(http.MethodPost, `{"action":"prepare"}`))
		secondDone <- report
	}()
	select {
	case <-secondDone:
		close(release)
		t.Fatal("second setup received the challenge before worker admission was installed")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	<-firstDone
	report := <-secondDone
	if report.Created || report.Challenge == nil || broker.expectations != 1 {
		t.Fatal("concurrent setup did not preserve one committed worker expectation")
	}
}
