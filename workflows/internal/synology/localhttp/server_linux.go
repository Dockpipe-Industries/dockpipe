//go:build linux

// Package localhttp provides the qualification services' local HTTP limits.
package localhttp

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type connection struct {
	net.Conn
	release func()
}

func (current *connection) Close() error {
	err := current.Conn.Close()
	current.release()
	return err
}

func ReserveConnection(current net.Conn, slots chan struct{}) net.Conn {
	select {
	case slots <- struct{}{}:
		var once sync.Once
		return &connection{
			Conn: current,
			release: func() {
				once.Do(func() {
					<-slots
				})
			},
		}
	default:
		_ = current.Close()
		return nil
	}
}

func AcceptLoopback(listener net.Listener, slots chan struct{}) (net.Conn, error) {
	for {
		current, err := listener.Accept()
		if err != nil {
			return nil, err
		}
		address, ok := current.RemoteAddr().(*net.TCPAddr)
		if !ok || !address.IP.IsLoopback() {
			_ = current.Close()
			continue
		}
		if bounded := ReserveConnection(current, slots); bounded != nil {
			return bounded, nil
		}
	}
}

type Listener struct {
	net.Listener
	Slots chan struct{}
}

func (listener *Listener) Accept() (net.Conn, error) {
	return AcceptLoopback(listener.Listener, listener.Slots)
}

func Server(handler http.Handler, maximumHeaderBytes int, timeout time.Duration) *http.Server {
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: time.Second, ReadTimeout: timeout,
		WriteTimeout: timeout, IdleTimeout: time.Second, MaxHeaderBytes: maximumHeaderBytes,
	}
	server.SetKeepAlivesEnabled(false)
	return server
}
