// Package monitor 服务器状态监控应用服务。
package monitor

import (
	"context"

	bizmonitor "github.com/smilex/smilex-admin-gin/internal/biz/monitor"
)

type Service struct {
	uc *bizmonitor.Usecase
}

func NewService(uc *bizmonitor.Usecase) *Service {
	return &Service{uc: uc}
}

// HostVO 主机静态信息
type HostVO struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelArch      string `json:"kernel_arch"`
	KernelVersion   string `json:"kernel_version"`
	BootTime        uint64 `json:"boot_time"` // unix 秒
}

// CPUVO 处理器指标
type CPUVO struct {
	ModelName string    `json:"model_name"`
	Cores     int       `json:"cores"`
	Percent   float64   `json:"percent"`
	PerCore   []float64 `json:"per_core"`
}

// MemVO 内存指标
type MemVO struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
}

// DiskVO 磁盘分区用量
type DiskVO struct {
	Device      string  `json:"device"`
	Mount       string  `json:"mount"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// NetVO 网卡累计流量与实时速率
type NetVO struct {
	Name      string  `json:"name"`
	BytesSent uint64  `json:"bytes_sent"`
	BytesRecv uint64  `json:"bytes_recv"`
	SendRate  float64 `json:"send_rate"` // B/s
	RecvRate  float64 `json:"recv_rate"` // B/s
}

// GoRuntimeVO Go 进程运行时指标
type GoRuntimeVO struct {
	Version      string  `json:"version"`
	Goroutines   int     `json:"goroutines"`
	HeapAlloc    uint64  `json:"heap_alloc"`
	SysMemory    uint64  `json:"sys_memory"`
	GCCount      int64   `json:"gc_count"`
	GCPauseMs    float64 `json:"gc_pause_ms"`
	ProcessStart int64   `json:"process_start"` // unix 秒
}

// ServerStatusVO 服务器状态快照
type ServerStatusVO struct {
	Time   int64       `json:"time"`   // unix 秒
	Uptime int64       `json:"uptime"` // 主机已运行秒数（由启动时间推算）
	Host   *HostVO     `json:"host"`
	CPU    CPUVO       `json:"cpu"`
	Memory MemVO       `json:"memory"`
	Disks  []DiskVO    `json:"disks"`
	Net    []NetVO     `json:"net"`
	Go     GoRuntimeVO `json:"go"`
}

// ServerStatus 组装服务器状态快照
func (s *Service) ServerStatus(ctx context.Context) (*ServerStatusVO, error) {
	st, err := s.uc.ServerStatus(ctx)
	if err != nil {
		return nil, err
	}
	vo := &ServerStatusVO{
		Time: st.Time,
		CPU: CPUVO{ModelName: st.CPU.ModelName, Cores: st.CPU.Cores,
			Percent: st.CPU.Percent, PerCore: st.CPU.PerCore},
		Memory: MemVO{Total: st.Memory.Total, Used: st.Memory.Used, Available: st.Memory.Available,
			UsedPercent: st.Memory.UsedPercent, SwapTotal: st.Memory.SwapTotal,
			SwapUsed: st.Memory.SwapUsed, SwapPercent: st.Memory.SwapPercent},
		Disks: make([]DiskVO, 0, len(st.Disks)),
		Net:   make([]NetVO, 0, len(st.Net)),
		Go: GoRuntimeVO{Version: st.Go.Version, Goroutines: st.Go.Goroutines,
			HeapAlloc: st.Go.HeapAlloc, SysMemory: st.Go.SysMemory,
			GCCount: st.Go.GCCount, GCPauseMs: st.Go.GCPauseMs, ProcessStart: st.Go.ProcessStart},
	}
	if st.Host != nil {
		vo.Host = &HostVO{Hostname: st.Host.Hostname, OS: st.Host.OS, Platform: st.Host.Platform,
			PlatformVersion: st.Host.PlatformVersion, KernelArch: st.Host.KernelArch,
			KernelVersion: st.Host.KernelVersion, BootTime: st.Host.BootTime}
		vo.Uptime = st.Time - int64(st.Host.BootTime)
		if vo.Uptime < 0 {
			vo.Uptime = 0
		}
	}
	for _, d := range st.Disks {
		vo.Disks = append(vo.Disks, DiskVO{Device: d.Device, Mount: d.Mount, Fstype: d.Fstype,
			Total: d.Total, Used: d.Used, Free: d.Free, UsedPercent: d.UsedPercent})
	}
	for _, n := range st.Net {
		vo.Net = append(vo.Net, NetVO{Name: n.Name, BytesSent: n.BytesSent, BytesRecv: n.BytesRecv,
			SendRate: n.SendRate, RecvRate: n.RecvRate})
	}
	return vo, nil
}
