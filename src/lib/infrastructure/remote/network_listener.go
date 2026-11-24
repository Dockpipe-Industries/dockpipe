package remote

import (
	"net"
	"sync"

	contract "dockpipe/src/lib/domain/remote"
)

// Header readers and idle HTTP connections count against this limit too. Two
// accepted sockets per stream allow bounded authentication/reconnection overlap.
type networkListener struct {
	net.Listener
	slots  chan struct{}
	closed chan struct{}
	once   sync.Once
}

func newNetworkListener(listener net.Listener) net.Listener {
	return &networkListener{Listener: listener, slots: make(chan struct{}, 2*contract.MaxNetworkNodeStreams), closed: make(chan struct{})}
}

func (listener *networkListener) Accept() (net.Conn, error) {
	select {
	case listener.slots <- struct{}{}:
	case <-listener.closed:
		return nil, net.ErrClosed
	}
	connection, err := listener.Listener.Accept()
	if err != nil {
		<-listener.slots
		return nil, err
	}
	return &networkAcceptedConn{Conn: connection, release: func() { <-listener.slots }}, nil
}

func (listener *networkListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return listener.Listener.Close()
}

type networkAcceptedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (connection *networkAcceptedConn) Close() error {
	err := connection.Conn.Close()
	connection.once.Do(connection.release)
	return err
}
