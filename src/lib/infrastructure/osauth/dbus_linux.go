//go:build linux && (amd64 || arm64)

package osauth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type busReply struct {
	Type string            `json:"type"`
	Data []json.RawMessage `json:"data"`
}

func decodeBusReply(raw []byte, expected string, output any) error {
	var reply busReply
	if json.Unmarshal(raw, &reply) != nil || reply.Type != expected || len(reply.Data) != 1 {
		return errors.New("invalid OS security D-Bus response")
	}
	return json.Unmarshal(reply.Data[0], output)
}

type desktopSession struct {
	locked bool
}

var errScreenSaverUnavailable = errors.New("desktop screen-lock integration is unavailable")

func readDesktopSession(ctx context.Context, run commandRunner, name string) (desktopSession, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	arguments := []string{"--system", "--json=short", "--timeout=2s", "call", "org.freedesktop.login1",
		"/org/freedesktop/login1", "org.freedesktop.login1.Manager", "GetSession", "s", name}
	raw, err := run(ctx, "/usr/bin/busctl", arguments...)
	var path string
	if err != nil || decodeBusReply(raw, "o", &path) != nil || !strings.HasPrefix(path, "/org/freedesktop/login1/session/") {
		return desktopSession{}, errors.New("OS login session lock integration is unavailable")
	}
	raw, err = run(ctx, "/usr/bin/busctl", "--system", "--json=short", "--timeout=2s", "call",
		"org.freedesktop.login1", path, "org.freedesktop.DBus.Properties", "GetAll", "s", "org.freedesktop.login1.Session")
	var properties map[string]struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err != nil || decodeBusReply(raw, "a{sv}", &properties) != nil {
		return desktopSession{}, errors.New("OS login session lock integration is unavailable")
	}
	var active, remote, locked bool
	readBool := func(name string, value *bool) bool {
		property, exists := properties[name]
		return exists && property.Type == "b" && json.Unmarshal(property.Data, value) == nil
	}
	if !readBool("Active", &active) || !readBool("Remote", &remote) || !readBool("LockedHint", &locked) || !active || remote {
		return desktopSession{}, errors.New("active local OS lock state is unavailable")
	}
	if locked {
		return desktopSession{locked: true}, nil
	}

	// Some lockers can lock a session without updating logind's
	// LockedHint even when logind advertises CanLock.
	desktopLocked, err := readScreenSaverLock(ctx, run, os.Getuid())
	if err == nil {
		return desktopSession{locked: desktopLocked}, nil
	}

	var canLock bool
	if errors.Is(err, errScreenSaverUnavailable) && readBool("CanLock", &canLock) && canLock {
		return desktopSession{}, nil
	}
	return desktopSession{}, err
}

func readScreenSaverLock(ctx context.Context, run commandRunner, uid int) (bool, error) {
	address := "--address=unix:path=/run/user/" + strconv.Itoa(uid) + "/bus"
	services := []struct{ name, path, contract string }{
		{"org.gnome.ScreenSaver", "/org/gnome/ScreenSaver", "org.gnome.ScreenSaver"},
		{"org.gnome.Shell", "/org/gnome/ScreenSaver", "org.gnome.ScreenSaver"},
		{"org.cinnamon.ScreenSaver", "/org/cinnamon/ScreenSaver", "org.cinnamon.ScreenSaver"},
		{"org.freedesktop.ScreenSaver", "/ScreenSaver", "org.freedesktop.ScreenSaver"},
		{"org.xfce.ScreenSaver", "/org/xfce/ScreenSaver", "org.xfce.ScreenSaver"},
	}
	available := false
	for _, service := range services {
		query, cancel := context.WithTimeout(ctx, time.Second)
		raw, err := run(query, "/usr/bin/busctl", address, "--json=short", "--auto-start=no", "--timeout=1s", "call",
			service.name, service.path, service.contract, "GetActive")
		cancel()

		var locked bool
		if err == nil && decodeBusReply(raw, "b", &locked) == nil {
			if locked {
				return true, nil
			}
			available = true
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
	}
	if available {
		return false, nil
	}
	return false, errScreenSaverUnavailable
}

func readGnomeIdle(ctx context.Context, run commandRunner, uid int) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	address := "--address=unix:path=/run/user/" + strconv.Itoa(uid) + "/bus"
	raw, err := run(ctx, "/usr/bin/busctl", address, "--json=short", "--auto-start=no", "--timeout=1s", "call",
		"org.gnome.Mutter.IdleMonitor", "/org/gnome/Mutter/IdleMonitor/Core", "org.gnome.Mutter.IdleMonitor", "GetIdletime")
	var milliseconds uint64
	if err != nil || decodeBusReply(raw, "t", &milliseconds) != nil || milliseconds > uint64((365*24*time.Hour)/time.Millisecond) {
		return 0, errors.New("desktop human-input idle time is unavailable")
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}
