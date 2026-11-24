//go:build darwin

package telemetry

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func darwinInterfaceRecord(flags int32, received, sent uint64) []byte {
	var header unix.IfMsghdr2
	header.Msglen = unix.SizeofIfMsghdr2
	header.Version, header.Type = unix.RTM_VERSION, unix.RTM_IFINFO2
	header.Flags = flags
	header.Data.Ibytes, header.Data.Obytes = received, sent
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(&header)), unix.SizeofIfMsghdr2)...)
}

func TestDarwinSystemNativeABIGuards(t *testing.T) {
	var header unix.IfMsghdr2
	var counters unix.IfData64
	var vm darwinVMStatistics64
	if unsafe.Sizeof(header) != 160 || unsafe.Offsetof(header.Data) != 32 || unsafe.Offsetof(counters.Ibytes) != 64 || unsafe.Offsetof(counters.Obytes) != 72 || unsafe.Sizeof(uintptr(0)) != 8 {
		t.Fatal("Darwin SDK network counter layout changed")
	}
	if unsafe.Sizeof(vm) != 160 || unsafe.Alignof(vm) != 8 || unsafe.Offsetof(vm.Decompressions) != 96 || unsafe.Offsetof(vm.SwappedPages) != 152 {
		t.Fatal("Darwin SDK VM statistics revisions or alignment changed")
	}
}

func TestDarwinSystemNetworkParsing(t *testing.T) {
	data := darwinInterfaceRecord(unix.IFF_UP, 20, 30)
	data = append(data, darwinInterfaceRecord(unix.IFF_LOOPBACK, 1000, 1000)...)
	data = append(data, darwinInterfaceRecord(0, 40, 50)...)
	received, sent, err := parseDarwinNetwork(context.Background(), data)
	if err != nil || received != 60 || sent != 80 {
		t.Fatalf("native cumulative interface totals: %d %d %v", received, sent, err)
	}
	for name, invalid := range map[string][]byte{
		"truncated":      data[:len(data)-1],
		"short header":   {1, 0, unix.RTM_VERSION},
		"zero length":    {0, 0, unix.RTM_VERSION, unix.RTM_IFINFO2},
		"short counters": {4, 0, unix.RTM_VERSION, unix.RTM_IFINFO2},
		"overflow":       append(darwinInterfaceRecord(0, math.MaxUint64, 0), darwinInterfaceRecord(0, 1, 0)...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseDarwinNetwork(context.Background(), invalid); err == nil {
				t.Fatal("invalid native network data accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := parseDarwinNetwork(ctx, data); err != context.Canceled {
		t.Fatal("cancelled network parsing continued")
	}
}

func TestDarwinSystemMachReturnCodesCountsAndPortCleanup(t *testing.T) {
	for _, mode := range []string{"success", "return-code", "short-count", "long-count", "cleanup-failure"} {
		t.Run(mode, func(t *testing.T) {
			released := 0
			api := &darwinMach{task: 3}
			api.hostSelf = func() uint32 { return 7 }
			api.deallocate = func(task, host uint32) int32 {
				if task != 3 || host != 7 {
					t.Fatal("wrong host right released")
				}
				released++
				if mode == "cleanup-failure" {
					return 5
				}
				return 0
			}
			api.sysctl = func(name string, output unsafe.Pointer, length *uintptr, _, _ uintptr) int32 {
				if name != "hw.logicalcpu" || *length != 4 {
					t.Fatal("unexpected CPU sysctl ABI")
				}
				*(*uint32)(output) = 8
				return 0
			}
			api.statistics = func(host uint32, flavor int32, output *uint32, count *uint32) int32 {
				if host != 7 || flavor != 3 || *count != 4 {
					t.Fatal("unexpected CPU Mach ABI")
				}
				copy(unsafe.Slice(output, 4), []uint32{1, 2, 3, 4})
				if mode == "short-count" {
					*count = 3
				}
				if mode == "long-count" {
					*count = 5
				}
				if mode == "return-code" {
					return 5
				}
				return 0
			}
			logical, ticks, err := (nativeDarwinSystem{api: api}).cpu()
			if released != 1 || (err == nil) != (mode == "success") {
				t.Fatalf("Mach failure/count/cleanup handling: released=%d err=%v", released, err)
			}
			if mode == "success" && (logical != 8 || ticks != [4]uint32{1, 2, 3, 4}) {
				t.Fatal("native CPU payload changed")
			}
		})
	}
}

func TestDarwinSystemNamedReadRejectsChangedSize(t *testing.T) {
	api := &darwinMach{sysctl: func(_ string, _ unsafe.Pointer, length *uintptr, _, _ uintptr) int32 {
		*length = 8
		return 0
	}}
	var value uint32
	if api.named("hw.logicalcpu", unsafe.Pointer(&value), 4) == nil {
		t.Fatal("kernel counter with incompatible ABI accepted")
	}
}

// Run this exact test on the paired Mac before claiming platform support. Its
// logs contain only aggregate counters, never process arguments or identities.
func TestDarwinSystemNativeCounters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := (NativeCollector{}).System(ctx)
	if err != nil || first.BootID == "" || first.CPUTotal == nil || first.CPUIdle == nil || first.MemoryTotal == nil || first.MemoryAvailable == nil || first.SwapTotal == nil || first.SwapFree == nil || first.DiskTotal == nil || first.DiskAvailable == nil || first.NetworkReceived == nil || first.NetworkSent == nil || first.UptimeSeconds == nil || first.Load1 == nil {
		t.Fatalf("required native Darwin counters unavailable: %#v %v", first, err)
	}
	// Unprivileged host_statistics replies may be cached for one second by XNU.
	time.Sleep(1100 * time.Millisecond)
	second, err := (NativeCollector{}).System(ctx)
	if err != nil || second.CPUTotal == nil || second.CPUIdle == nil || second.BootID != first.BootID || *second.CPUTotal <= *first.CPUTotal || *second.CPUIdle < *first.CPUIdle {
		t.Fatalf("native Darwin CPU counters did not advance consistently: first=%#v second=%#v err=%v", first, second, err)
	}
	t.Logf("logical_cpus=%d cpu_tick_delta=%d idle_tick_delta=%d memory_total=%d memory_available=%d disk_total=%d network_received=%d network_sent=%d uptime=%.3f load1=%.3f", second.LogicalCPUs,
		*second.CPUTotal-*first.CPUTotal, *second.CPUIdle-*first.CPUIdle, *first.MemoryTotal, *first.MemoryAvailable,
		*first.DiskTotal, *first.NetworkReceived, *first.NetworkSent, *first.UptimeSeconds, *first.Load1)
}

func TestDarwinSystemInterfaceWireFixture(t *testing.T) {
	// Independent offsets from Apple's if_msghdr2/if_data64 layout guard the
	// parser against accidentally reading 32-bit or packet counters as bytes.
	data := make([]byte, 160)
	binary.LittleEndian.PutUint16(data, 160)
	data[2], data[3] = unix.RTM_VERSION, unix.RTM_IFINFO2
	binary.LittleEndian.PutUint64(data[96:], 1<<40)
	binary.LittleEndian.PutUint64(data[104:], 1<<42)
	received, sent, err := parseDarwinNetwork(context.Background(), data)
	if err != nil || received != 1<<40 || sent != 1<<42 {
		t.Fatal("network ABI byte offsets do not match Apple interface counters")
	}
}

func TestDarwinSystemVMRevisionBoundsAndCleanup(t *testing.T) {
	for _, returned := range []uint32{23, 24, 38, 40, 41} {
		t.Run(fmt.Sprintf("count-%d", returned), func(t *testing.T) {
			released := 0
			api := &darwinMach{task: 3}
			api.hostSelf = func() uint32 { return 7 }
			api.deallocate = func(task, host uint32) int32 {
				released++
				return 0
			}
			api.sysctl = func(name string, output unsafe.Pointer, length *uintptr, _, _ uintptr) int32 {
				if name != "hw.memsize" || *length != 8 {
					t.Fatal("unexpected physical-memory sysctl ABI")
				}
				*(*uint64)(output) = 1 << 30
				return 0
			}
			api.pageSize = func(host uint32, output *uintptr) int32 {
				*output = 16384
				return 0
			}
			api.statistics64 = func(host uint32, flavor int32, output *uint32, count *uint32) int32 {
				if host != 7 || flavor != 4 || *count != 40 || uintptr(unsafe.Pointer(output))%8 != 0 {
					t.Fatal("VM statistics request does not match the bounded SDK ABI")
				}
				copy(unsafe.Slice(output, 4), []uint32{3, 4, 5, 6})
				*count = returned
				return 0
			}
			total, free, inactive, page, err := (nativeDarwinSystem{api: api}).memory()
			valid := returned >= 24 && returned <= 40
			if released != 1 || (err == nil) != valid {
				t.Fatalf("VM revision or port cleanup invalid: released=%d err=%v", released, err)
			}
			if valid && (total != 1<<30 || free != 3 || inactive != 5 || page != 16384) {
				t.Fatal("VM revision changed the stable native memory fields")
			}
		})
	}
}
