package system

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type CPUInfo struct {
	Info    []cpu.InfoStat  `json:"info"`
	Percent []float64       `json:"percent"`
	Times   []cpu.TimesStat `json:"times"`
}

type MemInfo struct {
	Virtual *mem.VirtualMemoryStat `json:"virtual"`
	Swap    *mem.SwapMemoryStat    `json:"swap"`
}

type DiskInfo struct {
	Partitions []disk.PartitionStat           `json:"partitions"`
	Usage      map[string]*disk.UsageStat     `json:"usage"`
	IOCounters map[string]disk.IOCountersStat `json:"io_counters"`
}

type NetInfo struct {
	Interfaces  []net.InterfaceStat  `json:"interfaces"`
	IOCounters  []net.IOCountersStat `json:"io_counters"`
	Connections []net.ConnectionStat `json:"connections,omitempty"`
}

type HostInfo struct {
	Info    *host.InfoStat         `json:"info"`
	Sensors []host.TemperatureStat `json:"sensors,omitempty"`
}

type ProcBrief struct {
	Pid        int32   `json:"pid"`
	Name       string  `json:"name"`
	Exe        string  `json:"exe,omitempty"`
	Cmdline    string  `json:"cmdline,omitempty"`
	Username   string  `json:"username,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemPercent float32 `json:"mem_percent,omitempty"`
	Status     string  `json:"status,omitempty"`
}

type ProcInfo struct {
	Count   int         `json:"count"`
	Top     []ProcBrief `json:"top"` // top N par CPU
	Sampled []ProcBrief `json:"sampled,omitempty"`
}

type All struct {
	CPU   *CPUInfo  `json:"cpu"`
	Mem   *MemInfo  `json:"mem"`
	Disk  *DiskInfo `json:"disk"`
	Net   *NetInfo  `json:"net"`
	Host  *HostInfo `json:"host"`
	Procs *ProcInfo `json:"processes"`
}

func WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, d)
}

func CollectCPU(ctx context.Context) (*CPUInfo, error) {
	var wg sync.WaitGroup
	var info []cpu.InfoStat
	var percent []float64
	var times []cpu.TimesStat
	var iErr, pErr, tErr error

	wg.Add(3)
	go func() {
		defer wg.Done()
		info, iErr = cpu.InfoWithContext(ctx)
	}()
	go func() {
		defer wg.Done()
		// interval=0 pour un “instantané” non-bloquant
		percent, pErr = cpu.PercentWithContext(ctx, 0, true)
	}()
	go func() {
		defer wg.Done()
		times, tErr = cpu.TimesWithContext(ctx, true)
	}()
	wg.Wait()

	if iErr != nil {
		return nil, iErr
	}
	if pErr != nil {
		return nil, pErr
	}
	if tErr != nil {
		return nil, tErr
	}

	return &CPUInfo{Info: info, Percent: percent, Times: times}, nil
}

func CollectMem(ctx context.Context) (*MemInfo, error) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	sm, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &MemInfo{Virtual: vm, Swap: sm}, nil
}

func CollectDisk(ctx context.Context) (*DiskInfo, error) {
	parts, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	usage := make(map[string]*disk.UsageStat)
	var io map[string]disk.IOCountersStat
	var ioErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for _, p := range parts {
			u, e := disk.UsageWithContext(ctx, p.Mountpoint)
			if e == nil {
				usage[p.Mountpoint] = u
			}
		}
	}()
	go func() {
		defer wg.Done()
		io, ioErr = disk.IOCountersWithContext(ctx)
	}()
	wg.Wait()
	if ioErr != nil {
		return nil, ioErr
	}

	return &DiskInfo{
		Partitions: parts,
		Usage:      usage,
		IOCounters: io,
	}, nil
}

func CollectNet(ctx context.Context, includeConnections bool) (*NetInfo, error) {
	ifaces, err := net.InterfacesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	io, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	n := &NetInfo{Interfaces: ifaces, IOCounters: io}
	if includeConnections {
		// ATTENTION: peut être coûteux et nécessiter des privilèges
		conns, _ := net.ConnectionsWithContext(ctx, "all")
		n.Connections = conns
	}
	return n, nil
}

func CollectHost(ctx context.Context) (*HostInfo, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	sensors, _ := host.SensorsTemperaturesWithContext(ctx) // best effort (Linux/root)
	return &HostInfo{Info: info, Sensors: sensors}, nil
}

func CollectProcesses(ctx context.Context, topN int) (*ProcInfo, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	type scored struct {
		brief ProcBrief
	}
	var wg sync.WaitGroup
	ch := make(chan scored, len(procs))

	for _, p := range procs {
		pp := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			name, _ := pp.NameWithContext(ctx)
			exe, _ := pp.ExeWithContext(ctx)
			cmd, _ := pp.CmdlineWithContext(ctx)
			user, _ := pp.UsernameWithContext(ctx)
			statuses, _ := pp.StatusWithContext(ctx)
			st := ""
			if len(statuses) > 0 {
				st = statuses[0]
			}
			cpuPct, _ := pp.CPUPercentWithContext(ctx)
			memPct, _ := pp.MemoryPercentWithContext(ctx)

			ch <- scored{brief: ProcBrief{
				Pid: pp.Pid, Name: name, Exe: exe, Cmdline: cmd, Username: user,
				CPUPercent: cpuPct, MemPercent: memPct, Status: st,
			}}
		}()
	}

	wg.Wait()
	close(ch)

	all := make([]ProcBrief, 0, len(procs))
	for s := range ch {
		all = append(all, s.brief)
	}

	// tri simple par CPU desc (in-place)
	// (pas de dépendance supplémentaire)
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].CPUPercent > all[i].CPUPercent {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if topN > len(all) {
		topN = len(all)
	}
	top := all[:topN]

	return &ProcInfo{
		Count: len(all),
		Top:   top,
		// pour limiter la taille, on n’inclut pas toute la liste par défaut
	}, nil
}

func CollectAll(ctx context.Context) (*All, error) {
	ctx, cancel := WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var cpuI *CPUInfo
	var memI *MemInfo
	var diskI *DiskInfo
	var netI *NetInfo
	var hostI *HostInfo
	var procI *ProcInfo
	var cErr, mErr, dErr, nErr, hErr, pErr error

	wg.Add(6)
	go func() { defer wg.Done(); cpuI, cErr = CollectCPU(ctx) }()
	go func() { defer wg.Done(); memI, mErr = CollectMem(ctx) }()
	go func() { defer wg.Done(); diskI, dErr = CollectDisk(ctx) }()
	go func() { defer wg.Done(); netI, nErr = CollectNet(ctx, false) }()
	go func() { defer wg.Done(); hostI, hErr = CollectHost(ctx) }()
	go func() { defer wg.Done(); procI, pErr = CollectProcesses(ctx, 15) }()
	wg.Wait()

	// on renvoie ce qui est dispo même si une partie a échoué
	return &All{
		CPU: cpuI, Mem: memI, Disk: diskI, Net: netI, Host: hostI, Procs: procI,
	}, firstErr(cErr, mErr, dErr, nErr, hErr, pErr)
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
