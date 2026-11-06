//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func endpointFixture(t *testing.T) (string, *tls.Config) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := initializeEndpoint(root); err != nil {
		t.Fatal(err)
	}
	configuration, err := loadEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, configuration
}

func TestEndpointInitializationPreservesPrivateMaterialAndPublicPin(t *testing.T) {
	root, configuration := endpointFixture(t)
	keyPath, certificatePath, err := endpointPaths(root, false)
	if err != nil {
		t.Fatal(err)
	}
	key, err := endpointFile(keyPath, 0600, os.Geteuid())
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := endpointFile(certificatePath, 0644, os.Geteuid())
	if err != nil {
		t.Fatal(err)
	}
	if initializeEndpoint(root) != nil {
		t.Fatal("valid endpoint pin was not reusable")
	}
	keyAfter, _ := os.ReadFile(keyPath)
	certificateAfter, _ := os.ReadFile(certificatePath)
	if !bytes.Equal(key, keyAfter) || !bytes.Equal(certificate, certificateAfter) {
		t.Fatal("endpoint reinitialization changed established material")
	}
	leaf := configuration.Certificates[0].Leaf
	if leaf.VerifyHostname("localhost") != nil || leaf.VerifyHostname("127.0.0.1") != nil || configuration.MinVersion != tls.VersionTLS12 {
		t.Fatal("endpoint identity or minimum TLS version is invalid")
	}
	if _, err := endpointFile(keyPath, 0600, os.Geteuid()+1); err == nil {
		t.Fatal("foreign-owned endpoint material would be accepted")
	}
}

func TestEndpointInitializationHonorsExplicitModesUnderInstallerUmask(t *testing.T) {
	previous := syscall.Umask(0077)
	defer syscall.Umask(previous)
	root, _ := endpointFixture(t)
	if err := endpointDirectory(filepath.Join(root, publicEndpointDirectory), 0755, false); err != nil {
		t.Fatal(err)
	}
	_, certificatePath, _ := endpointPaths(root, false)
	if _, err := endpointFile(certificatePath, 0644, os.Geteuid()); err != nil {
		t.Fatal("public pin inherited the private installer umask")
	}
}

func TestEndpointLoadRejectsMissingLinkedAndLooseMaterial(t *testing.T) {
	for _, scenario := range []string{"missing-key", "loose-key", "linked-key", "linked-private-directory", "private-public-directory"} {
		t.Run(scenario, func(t *testing.T) {
			root, _ := endpointFixture(t)
			keyPath, _, _ := endpointPaths(root, false)
			switch scenario {
			case "missing-key":
				if err := os.Remove(keyPath); err != nil {
					t.Fatal(err)
				}
			case "loose-key":
				if err := os.Chmod(keyPath, 0644); err != nil {
					t.Fatal(err)
				}
			case "linked-key":
				if err := os.Rename(keyPath, keyPath+".actual"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(keyPath+".actual", keyPath); err != nil {
					t.Fatal(err)
				}
			case "linked-private-directory":
				path := filepath.Join(root, privateEndpointDirectory)
				if err := os.Rename(path, path+".actual"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".actual", path); err != nil {
					t.Fatal(err)
				}
			case "private-public-directory":
				if err := os.Chmod(filepath.Join(root, publicEndpointDirectory), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := loadEndpoint(root); err == nil {
				t.Fatal("unsafe endpoint material was loaded")
			}
			if initializeEndpoint(root) == nil {
				t.Fatal("unsafe existing endpoint material was silently replaced")
			}
		})
	}
}

func TestPinnedTLSRejectsForgedListenerBeforeCredentials(t *testing.T) {
	root, trusted := endpointFixture(t)
	_, forged := endpointFixture(t)
	_, certificatePath, _ := endpointPaths(root, false)
	publicCertificate, err := os.ReadFile(certificatePath)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(publicCertificate) {
		t.Fatal("public endpoint pin did not parse")
	}
	var selected atomic.Pointer[tls.Certificate]
	selected.Store(&trusted.Certificates[0])
	var requests atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return selected.Load(), nil
	}}
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{DisableKeepAlives: true, TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	response, err := client.Get(server.URL + "/health")
	if err != nil {
		t.Fatal("valid pinned endpoint was rejected")
	}
	response.Body.Close()
	if response.StatusCode != 204 || requests.Load() != 1 {
		t.Fatal("valid pinned health check failed")
	}
	// Switch the same address/port to a different private key and certificate.
	// A spoofed 204 can never arrive: TLS must fail before any HTTP credentials.
	selected.Store(&forged.Certificates[0])
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/report", nil)
	request.Header.Set("Cookie", "id=credential-that-must-not-reach-forged-listener")
	request.Header.Set("X-Syno-Token", "token-that-must-not-reach-forged-listener")
	response, err = client.Do(request)
	if err == nil {
		response.Body.Close()
		t.Fatal("forged same-port TLS endpoint was accepted")
	}
	if requests.Load() != 1 {
		t.Fatal("HTTP credentials reached the forged listener")
	}
}

func TestAuthenticatedWorkerFailureReasonsAreAllowlisted(t *testing.T) {
	for _, reason := range []string{"authentication_unavailable", "local_account_ineligible", "delegation_failed", "untrusted-secret-error"} {
		handler := identityHandler(func(_ context.Context, _ []string) (workerResponse, error) {
			return workerResponse{Error: reason}, nil
		})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, identityRequest())
		if response.Code != 503 || bytes.Contains(response.Body.Bytes(), []byte("secret")) {
			t.Fatal("worker diagnostics exposed an untrusted error")
		}
		if reason != "untrusted-secret-error" && !bytes.Contains(response.Body.Bytes(), []byte(reason)) {
			t.Fatal("allowlisted failure reason was lost")
		}
	}
}
