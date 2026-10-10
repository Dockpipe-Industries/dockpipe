//go:build linux

package pipelang

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The optional conformance representation service supplies bytes only. The
// caller computes the current build key and executes fresh current oracles.
func acquireGeneratedRepresentation(root, key string) (*os.File, error) {
	address := os.Getenv("PIPELANG_NATIVE_SOCKET")
	if address == "" || root == "" {
		return nil, nil
	}
	expected, err := generatedArtifactExpectedDigest(root, key)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	connection, err := net.DialTimeout("unixpacket", address, 25*time.Second)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	socket := connection.(*net.UnixConn)
	if err := socket.SetDeadline(time.Now().Add(25 * time.Second)); err != nil {
		return nil, err
	}
	request, err := json.Marshal(map[string]string{"cache": root, "key": key, "sha256": expected})
	if err != nil {
		return nil, err
	}
	if _, err := socket.Write(request); err != nil {
		return nil, err
	}
	data, control := make([]byte, 4096), make([]byte, unix.CmsgSpace(4))
	n, oob, flags, _, err := socket.ReadMsgUnix(data, control)
	if err != nil {
		return nil, err
	}
	messages, err := unix.ParseSocketControlMessage(control[:oob])
	if err != nil {
		return nil, err
	}
	var descriptors []int
	for _, message := range messages {
		rights, err := unix.ParseUnixRights(&message)
		if err != nil {
			for _, fd := range descriptors {
				unix.Close(fd)
			}
			return nil, err
		}
		descriptors = append(descriptors, rights...)
	}
	defer func() {
		for _, fd := range descriptors {
			unix.Close(fd)
		}
	}()
	if string(data[:n]) == "fallback" && len(descriptors) == 0 {
		return nil, nil
	}
	if string(data[:n]) != "ok" || len(descriptors) != 1 || flags&(unix.MSG_TRUNC|unix.MSG_CTRUNC) != 0 {
		return nil, fmt.Errorf("native representation rejected: %s", data[:n])
	}
	fd := descriptors[0]
	unix.CloseOnExec(fd)
	const required = unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_SEAL
	seals, err := unix.FcntlInt(uintptr(fd), unix.F_GET_SEALS, 0)
	if err != nil || seals&required != required {
		return nil, fmt.Errorf("native representation is not sealed")
	}
	file := os.NewFile(uintptr(fd), "pipelang-native-representation")
	descriptors = nil
	success := false
	defer func() {
		if !success {
			file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 16<<20 {
		return nil, fmt.Errorf("native representation exceeds bounds")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return nil, fmt.Errorf("native representation digest mismatch")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	success = true
	return file, nil
}

func TestGeneratedRepresentationRejectsUnsealedAndWrongBytes(t *testing.T) {
	for _, mode := range []string{"unsealed", "wrong-bytes", "valid"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			key := "request"
			data := []byte("exact native bytes")
			sum := sha256.Sum256(data)
			expected := hex.EncodeToString(sum[:])
			if err := os.Mkdir(filepath.Join(root, key), 0700); err != nil {
				t.Fatal(err)
			}
			record, _ := json.Marshal(generatedCacheRecord{generatedCacheVersion, key, expected})
			if err := os.WriteFile(filepath.Join(root, key, "record.json"), record, 0600); err != nil {
				t.Fatal(err)
			}
			// Durable campaign paths can exceed sockaddr_un's pathname limit.
			// Address the same private directory through its open descriptor.
			rootDirectory, err := os.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer rootDirectory.Close()
			address := fmt.Sprintf("/proc/self/fd/%d/socket", rootDirectory.Fd())
			listener, err := net.ListenUnix("unixpacket", &net.UnixAddr{Name: address, Net: "unixpacket"})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan error, 1)
			go func() {
				c, err := listener.AcceptUnix()
				if err != nil {
					done <- err
					return
				}
				defer c.Close()
				c.SetDeadline(time.Now().Add(2 * time.Second))
				request := make([]byte, 4096)
				if _, err = c.Read(request); err != nil {
					done <- err
					return
				}
				fd, err := unix.MemfdCreate("transport-probe", unix.MFD_ALLOW_SEALING)
				if err != nil {
					done <- err
					return
				}
				defer unix.Close(fd)
				if mode == "wrong-bytes" {
					data = []byte("wrong native bytes")
				}
				if _, err = unix.Write(fd, data); err != nil {
					done <- err
					return
				}
				unix.Seek(fd, 0, 0)
				if mode != "unsealed" {
					_, err = unix.FcntlInt(uintptr(fd), unix.F_ADD_SEALS, unix.F_SEAL_SEAL|unix.F_SEAL_WRITE|unix.F_SEAL_GROW|unix.F_SEAL_SHRINK)
				}
				if err == nil {
					_, _, err = c.WriteMsgUnix([]byte("ok"), unix.UnixRights(fd), nil)
				}
				done <- err
			}()
			t.Setenv("PIPELANG_NATIVE_SOCKET", address)
			file, err := acquireGeneratedRepresentation(root, key)
			if file != nil {
				file.Close()
			}
			if (err == nil) != (mode == "valid") {
				t.Fatalf("%s: file=%v err=%v", mode, file, err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
