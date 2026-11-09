//go:build darwin

package telemetry

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// Apple ABI references:
// xnu/osfmk/mach/{host_info.h,vm_statistics.h},
// xnu/libsyscall/mach/mach/mach_init.h and xnu/bsd/{sys/sysctl.h,net/if.h}.
// Mach ports, natural_t, counts and kern_return_t are 32-bit; vm_size_t and
// size_t are pointer-sized on the supported Darwin arm64/amd64 architectures.
const (
	darwinHostCPULoadInfo = 3
	darwinHostVMInfo64    = 4
	darwinNetworkBytes    = 256 * 1024 // Same bounded inventory budget as Linux.
)

// vm_statistics64_data_t through HOST_VM_INFO64_REV2. Earlier kernel
// revisions return a shorter count; the requested count bounds kernel writes.
// See xnu/osfmk/mach/vm_statistics.h and host_info.h, not a guessed spare tail.
type darwinVMStatistics64 struct {
	Free, Active, Inactive, Wired                      uint32
	ZeroFill, Reactivations, Pageins, Pageouts, Faults uint64
	CowFaults, Lookups, Hits, Purges                   uint64
	Purgeable, Speculative                             uint32
	Decompressions, Compressions, Swapins, Swapouts    uint64
	CompressorPages, Throttled, External, Internal     uint32
	TotalUncompressedPages, SwappedPages               uint64
}

type darwinMach struct {
	hostSelf     func() uint32
	statistics   func(uint32, int32, *uint32, *uint32) int32
	statistics64 func(uint32, int32, *uint32, *uint32) int32
	pageSize     func(uint32, *uintptr) int32
	deallocate   func(uint32, uint32) int32
	sysctl       func(string, unsafe.Pointer, *uintptr, uintptr, uintptr) int32
	loadavg      func(*float64, int32) int32
	task         uint32
}

var darwinMachOnce = sync.OnceValues(loadDarwinMach)

func loadDarwinMach() (*darwinMach, error) {
	handle, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, errors.New("Darwin system library unavailable")
	}
	api := &darwinMach{}
	for _, binding := range []struct {
		name        string
		destination any
	}{
		{"mach_host_self", &api.hostSelf},
		{"host_statistics", &api.statistics},
		{"host_statistics64", &api.statistics64},
		{"host_page_size", &api.pageSize},
		{"mach_port_deallocate", &api.deallocate},
		{"sysctlbyname", &api.sysctl},
		{"getloadavg", &api.loadavg},
	} {
		symbol, err := purego.Dlsym(handle, binding.name)
		if err != nil {
			_ = purego.Dlclose(handle)
			return nil, errors.New("Darwin system library lacks a required counter API")
		}
		purego.RegisterFunc(binding.destination, symbol)
	}
	// mach_task_self() is a public macro over this cached port, not a function.
	self, err := purego.Dlsym(handle, "mach_task_self_")
	if err != nil || self == 0 {
		_ = purego.Dlclose(handle)
		return nil, errors.New("Darwin task port unavailable")
	}
	api.task = *(*uint32)(unsafe.Pointer(self))
	if api.task == 0 {
		_ = purego.Dlclose(handle)
		return nil, errors.New("Darwin task port is null")
	}
	// Match existing native library ownership: successful bindings remain mapped
	// for process lifetime, so a concurrent sample cannot call unloaded symbols.
	return api, nil
}

func (api *darwinMach) withHost(collect func(uint32) error) (err error) {
	host := api.hostSelf()
	if host == 0 {
		return errors.New("Darwin host port is null")
	}
	defer func() {
		if code := api.deallocate(api.task, host); code != 0 {
			err = errors.Join(err, fmt.Errorf("Darwin host port release failed: %d", code))
		}
	}()
	return collect(host)
}

func (api *darwinMach) named(name string, output unsafe.Pointer, expected uintptr) error {
	length := expected
	// Every queried name has a fixed-size ABI (or the fixed 37-byte boot UUID).
	// Do not perform a size query, allocate returned sizes, or retry kernel reads.
	if api.sysctl(name, output, &length, 0, 0) != 0 || length != expected {
		return errors.New("Darwin system counter unavailable or has an unexpected size")
	}
	return nil
}

func (NativeCollector) System(ctx context.Context) (SystemResources, error) {
	if err := ctx.Err(); err != nil {
		return SystemResources{}, err
	}
	api, err := darwinMachOnce()
	if err != nil {
		return SystemResources{Notices: []string{err.Error()}}, ctx.Err()
	}
	return collectDarwinSystem(ctx, nativeDarwinSystem{api: api})
}

type nativeDarwinSystem struct{ api *darwinMach }

func (source nativeDarwinSystem) boot() (string, float64, error) {
	var uuid [37]byte
	var boot unix.Timeval
	if err := source.api.named("kern.bootsessionuuid", unsafe.Pointer(&uuid[0]), uintptr(len(uuid))); err != nil {
		return "", 0, err
	}
	if err := source.api.named("kern.boottime", unsafe.Pointer(&boot), unsafe.Sizeof(boot)); err != nil {
		return "", 0, err
	}
	identity := strings.TrimRight(string(uuid[:]), "\x00")
	if len(identity) != 36 || uuid[36] != 0 || boot.Sec <= 0 || boot.Usec < 0 || boot.Usec >= 1000000 {
		return "", 0, errors.New("invalid Darwin boot identity or time")
	}
	return identity, time.Since(time.Unix(boot.Sec, int64(boot.Usec)*1000)).Seconds(), nil
}

func (source nativeDarwinSystem) cpu() (uint32, [4]uint32, error) {
	var ticks [4]uint32
	var logical uint32
	if err := source.api.named("hw.logicalcpu", unsafe.Pointer(&logical), unsafe.Sizeof(logical)); err != nil {
		return 0, ticks, err
	}
	err := source.api.withHost(func(host uint32) error {
		count := uint32(len(ticks))
		if code := source.api.statistics(host, darwinHostCPULoadInfo, &ticks[0], &count); code != 0 || count != uint32(len(ticks)) {
			return errors.New("invalid Darwin cumulative CPU counters")
		}
		return nil
	})
	return logical, ticks, err
}

func (source nativeDarwinSystem) memory() (uint64, uint64, uint64, uint64, error) {
	var total uint64
	if err := source.api.named("hw.memsize", unsafe.Pointer(&total), unsafe.Sizeof(total)); err != nil {
		return 0, 0, 0, 0, err
	}
	// Mach counts are in 32-bit integer_t units. REV0 ends immediately
	// before decompressions; REV2 includes the entire public SDK structure.
	var vm darwinVMStatistics64
	minimumCount := uint32(unsafe.Offsetof(vm.Decompressions) / 4)
	maximumCount := uint32(unsafe.Sizeof(vm) / 4)
	var page uintptr
	err := source.api.withHost(func(host uint32) error {
		if code := source.api.pageSize(host, &page); code != 0 || page == 0 {
			return errors.New("Darwin page size unavailable")
		}
		count := maximumCount
		if code := source.api.statistics64(host, darwinHostVMInfo64, &vm.Free, &count); code != 0 || count < minimumCount || count > maximumCount {
			return errors.New("invalid Darwin memory counter revision")
		}
		return nil
	})
	return total, uint64(vm.Free), uint64(vm.Inactive), uint64(page), err
}

func (source nativeDarwinSystem) swap() (uint64, uint64, error) {
	var usage struct {
		Total, Available, Used uint64
		PageSize               uint32
		Encrypted              int32
	}
	if err := source.api.named("vm.swapusage", unsafe.Pointer(&usage), unsafe.Sizeof(usage)); err != nil {
		return 0, 0, err
	}
	if usage.Used > usage.Total {
		return 0, 0, errors.New("invalid Darwin swap usage")
	}
	return usage.Total, usage.Available, nil
}

func (nativeDarwinSystem) disk() (uint64, uint64, error) {
	var disk unix.Statfs_t
	if err := unix.Statfs("/", &disk); err != nil {
		return 0, 0, err
	}
	if disk.Bsize == 0 || disk.Blocks > math.MaxUint64/uint64(disk.Bsize) || disk.Bavail > disk.Blocks {
		return 0, 0, errors.New("invalid Darwin root-volume capacity")
	}
	return disk.Blocks * uint64(disk.Bsize), disk.Bavail * uint64(disk.Bsize), nil
}

func (nativeDarwinSystem) network(ctx context.Context) (uint64, uint64, error) {
	mib := [6]int32{unix.CTL_NET, unix.AF_ROUTE, 0, 0, unix.NET_RT_IFLIST2, 0}
	data := make([]byte, darwinNetworkBytes)
	length := uintptr(len(data))
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&length)), 0, 0)
	if errno != 0 || length > uintptr(len(data)) {
		return 0, 0, errors.New("Darwin network inventory unavailable or exceeds 256 KiB")
	}
	return parseDarwinNetwork(ctx, data[:length])
}

func parseDarwinNetwork(ctx context.Context, data []byte) (uint64, uint64, error) {
	var received, sent uint64
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		if len(data) < 4 {
			return 0, 0, errors.New("truncated Darwin interface header")
		}
		length := int(binary.LittleEndian.Uint16(data))
		if length < 4 || length > len(data) || data[2] != unix.RTM_VERSION {
			return 0, 0, errors.New("invalid Darwin interface record")
		}
		if data[3] == unix.RTM_IFINFO2 {
			if length < unix.SizeofIfMsghdr2 {
				return 0, 0, errors.New("truncated Darwin interface counters")
			}
			var header unix.IfMsghdr2
			copy(unsafe.Slice((*byte)(unsafe.Pointer(&header)), unix.SizeofIfMsghdr2), data[:unix.SizeofIfMsghdr2])
			if header.Flags&unix.IFF_LOOPBACK == 0 {
				if header.Data.Ibytes > math.MaxUint64-received || header.Data.Obytes > math.MaxUint64-sent {
					return 0, 0, errors.New("Darwin network counters overflow")
				}
				received += header.Data.Ibytes
				sent += header.Data.Obytes
			}
		}
		data = data[length:]
	}
	return received, sent, nil
}

func (source nativeDarwinSystem) load() (float64, error) {
	var average float64
	if source.api.loadavg(&average, 1) != 1 {
		return 0, errors.New("Darwin load average unavailable")
	}
	return average, nil
}
