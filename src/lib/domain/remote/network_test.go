package remote

import "testing"

func TestNetworkAddressContract(t *testing.T) {
	for _, address := range []string{"localhost:80", "example.com:443", "127.0.0.1:3000", "192.168.1.2:8080", "[::1]:443"} {
		if err := ValidateNetworkAddress(address); err != nil {
			t.Errorf("valid worker destination %q: %v", address, err)
		}
	}
	for _, address := range []string{"localhost", "localhost:0", "localhost:65536", "localhost:080", "http://localhost:80", "user@host:80", "bad\rhost:80", "[::1%eth0]:80", "-bad:80", "bad..host:80", "localhost:unix"} {
		if ValidateNetworkAddress(address) == nil {
			t.Errorf("invalid destination admitted: %q", address)
		}
	}
}
