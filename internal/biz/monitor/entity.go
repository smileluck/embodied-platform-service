// Package monitor 服务器状态监控领域：系统指标（CPU/内存/磁盘/网络/主机）+ Go 进程运行时。
package monitor

import (
	"context"
	"errors"
	"time"
)

// ErrCollectFailed 指标采集失败（系统 /proc 或 sysctl 不可读等系统性故障）
var ErrCollectFailed = errors.New("monitor: collect failed")

// HostInfo 主机静态信息（变化频率低，usecase 内缓存）
type HostInfo struct {
	Hostname        string
	OS              string // darwin / linux / windows
	Platform        string // 发行版：Ubuntu / macOS ...
	PlatformVersion string
	KernelArch      string // arm64 / amd64
	KernelVersion   string
	BootTime        uint64 // unix 秒
}

// CPUInfo 处理器指标（使用率为采样器固定 3s 窗口的差值平均）
type CPUInfo struct {
	ModelName string
	Cores     int       // 逻辑核数
	Percent   float64   // 总体使用率 0-100
	PerCore   []float64 // 每核使用率 0-100
}

// MemInfo 内存指标
type MemInfo struct {
	Total       uint64
	Used        uint64
	Available   uint64
	UsedPercent float64
	SwapTotal   uint64
	SwapUsed    uint64
	SwapPercent float64
}

// DiskInfo 磁盘分区用量
type DiskInfo struct {
	Device      string
	Mount       string
	Fstype      string
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// NetIO 网卡累计流量与实时速率（速率为采样器固定 3s 窗口的差值）
type NetIO struct {
	Name      string
	BytesSent uint64  // 累计发送字节
	BytesRecv uint64  // 累计接收字节
	SendRate  float64 // 发送速率 B/s
	RecvRate  float64 // 接收速率 B/s
}

// GoRuntime 本进程 Go 运行时指标
type GoRuntime struct {
	Version      string
	Goroutines   int
	HeapAlloc    uint64  // 堆内对象占用
	SysMemory    uint64  // 向系统申请的内存
	GCCount      int64   // 启动以来 GC 次数
	GCPauseMs    float64 // GC 累计停顿毫秒
	ProcessStart int64   // 进程启动 unix 秒
}

// ServerStatus 服务器状态快照（CPU/网络读采样器缓存，其余请求时采集）
type ServerStatus struct {
	Time   int64 // 快照 unix 秒
	Host   *HostInfo
	CPU    CPUInfo
	Memory MemInfo
	Disks  []DiskInfo
	Net    []NetIO
	Go     GoRuntime
}

// Snapshot 监控历史快照（60s 一点，保留期自动清理）
type Snapshot struct {
	Ts          time.Time `json:"ts"`          // 采样时刻
	CPUPercent  float64   `json:"cpu_percent"` // 总 CPU 使用率（3s 窗口差值）
	MemPercent  float64   `json:"mem_percent"` // 内存使用率
	SwapPercent float64   `json:"swap_percent"`
	NetSendRate float64   `json:"net_send_rate"` // 全部网卡合计 B/s
	NetRecvRate float64   `json:"net_recv_rate"`
}

// SnapshotRepo 历史快照仓储接口
type SnapshotRepo interface {
	SaveSnapshot(ctx context.Context, s *Snapshot) error
	// ListSnapshots 时间升序返回区间内快照（最多 limit 点）
	ListSnapshots(ctx context.Context, since time.Time, limit int) ([]*Snapshot, error)
	CleanupSnapshotsBefore(ctx context.Context, before time.Time) error
}
