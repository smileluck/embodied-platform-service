package monitor

import (
	"context"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
)

const (
	// sampleInterval 差值采样窗口：CPU% 与网卡速率依赖两次采样差值，
	// 固定周期使速率计算与前端轮询频率解耦（轮询只读快照）
	sampleInterval = 3 * time.Second
	// hostCacheTTL 主机静态信息缓存时长
	hostCacheTTL = 5 * time.Minute
)

// procStart 进程启动时间（main 之前的近似值，用于展示进程运行时长）
var procStart = time.Now().Unix()

// netNameSkip 不展示的回环网卡（各平台命名不同）
var netNameSkip = map[string]bool{"lo": true, "lo0": true}

// diskFstypeSkip 虚拟/伪文件系统分区不展示（overlay 除外：容器部署时它就是真实根分区）
var diskFstypeSkip = map[string]bool{
	"devtmpfs": true, "devfs": true, "proc": true, "sysfs": true,
	"tmpfs": true, "squashfs": true, "autofs": true, "nullfs": true,
	"iso9660": true, "udf": true,
}

// Usecase 服务器状态监控用例：
// CPU 使用率与网络速率由后台采样器固定 3s 窗口周期采样存内存快照（首次采样建立基线，
// 采样器是 cpu.Percent 的唯一调用方，避免基线窗口被请求打乱）；内存/磁盘等低开销指标
// 请求时实时采集；主机静态信息缓存 5 分钟。
type Usecase struct {
	snapRepo  SnapshotRepo
	mu        sync.RWMutex
	cpuPct    float64
	cpuPerCPU []float64
	netIO     []NetIO
	lastNet   map[string]gnet.IOCountersStat
	lastNetAt time.Time

	host   *HostInfo
	hostAt time.Time
	stop   chan struct{}
}

// NewUsecase 启动后台采样器（3s 差值）与历史落库循环（60s 一点，保留 7 天）；
// snapRepo 可为 nil（测试/无库场景退化为纯实时）。返回清理函数停止 goroutine。
func NewUsecase(snaps SnapshotRepo) (*Usecase, func()) {
	uc := &Usecase{snapRepo: snaps, stop: make(chan struct{})}
	go uc.sampleLoop()
	if snaps != nil {
		go uc.snapshotLoop()
	}
	return uc, func() { close(uc.stop) }
}

const (
	snapshotInterval = time.Minute // 历史落库间隔
	snapshotKeepDays = 7           // 历史保留天数
)

// snapshotLoop 每分钟落一帧当前快照；每小时清理过期历史
func (uc *Usecase) snapshotLoop() {
	snap := func() {
		s := uc.currentSnapshot()
		if err := uc.snapRepo.SaveSnapshot(context.Background(), s); err != nil {
			logger.Warn("monitor snapshot save failed", zap.Error(err))
		}
	}
	cleanup := func() {
		if err := uc.snapRepo.CleanupSnapshotsBefore(context.Background(), time.Now().AddDate(0, 0, -snapshotKeepDays)); err != nil {
			logger.Warn("monitor snapshot cleanup failed", zap.Error(err))
		}
	}
	snap()
	t := time.NewTicker(snapshotInterval)
	hourly := time.NewTicker(time.Hour)
	defer t.Stop()
	defer hourly.Stop()
	for {
		select {
		case <-uc.stop:
			return
		case <-t.C:
			snap()
		case <-hourly.C:
			cleanup()
		}
	}
}

// currentSnapshot 从内存采样值组帧
func (uc *Usecase) currentSnapshot() *Snapshot {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	s := &Snapshot{Ts: time.Now(), CPUPercent: uc.cpuPct}
	if st, err := mem.VirtualMemory(); err == nil {
		s.MemPercent = st.UsedPercent
	}
	if sw, err := mem.SwapMemory(); err == nil {
		s.SwapPercent = sw.UsedPercent
	}
	for _, n := range uc.netIO {
		s.NetSendRate += n.SendRate
		s.NetRecvRate += n.RecvRate
	}
	return s
}

// LatestSnapshot 当前最新采样帧（告警通知分发器越限评估用；与历史落库同源）
func (uc *Usecase) LatestSnapshot() *Snapshot { return uc.currentSnapshot() }

// History 历史快照（时间升序；hours 上限 72，最多 5000 点）
func (uc *Usecase) History(ctx context.Context, hours int) ([]*Snapshot, error) {
	if uc.snapRepo == nil {
		return nil, ErrCollectFailed
	}
	if hours <= 0 || hours > 72 {
		hours = 24
	}
	return uc.snapRepo.ListSnapshots(ctx, time.Now().Add(-time.Duration(hours)*time.Hour), 5000)
}

func (uc *Usecase) sampleLoop() {
	// 首轮建立差值基线（cpu.Percent 首次调用无上次值返回空，网卡需要累计值底数）
	uc.sampleCPU()
	uc.sampleNet()
	t := time.NewTicker(sampleInterval)
	defer t.Stop()
	for {
		select {
		case <-uc.stop:
			return
		case <-t.C:
			uc.sampleCPU()
			uc.sampleNet()
		}
	}
}

// sampleCPU 采一次 CPU 使用率（gopsutil 非阻塞模式内部以包级上次调用为基线）
func (uc *Usecase) sampleCPU() {
	total, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warn("monitor: sample cpu failed", zap.Error(err))
		return
	}
	per, err := cpu.Percent(0, true)
	if err != nil {
		logger.Warn("monitor: sample per-cpu failed", zap.Error(err))
		return
	}
	uc.mu.Lock()
	if len(total) > 0 {
		uc.cpuPct = total[0]
	}
	uc.cpuPerCPU = per
	uc.mu.Unlock()
}

// sampleNet 采一次网卡累计字节数，与上次差值求速率（B/s）
func (uc *Usecase) sampleNet() {
	counters, err := gnet.IOCounters(true)
	if err != nil {
		logger.Warn("monitor: sample net failed", zap.Error(err))
		return
	}
	now := time.Now()
	uc.mu.Lock()
	defer uc.mu.Unlock()
	elapsed := now.Sub(uc.lastNetAt).Seconds()
	last := uc.lastNet
	ios := make([]NetIO, 0, len(counters))
	next := make(map[string]gnet.IOCountersStat, len(counters))
	for _, c := range counters {
		next[c.Name] = c
		if netNameSkip[c.Name] {
			continue
		}
		io := NetIO{Name: c.Name, BytesSent: c.BytesSent, BytesRecv: c.BytesRecv}
		if elapsed > 0 && last != nil {
			if prev, ok := last[c.Name]; ok {
				io.SendRate = float64(c.BytesSent-prev.BytesSent) / elapsed
				io.RecvRate = float64(c.BytesRecv-prev.BytesRecv) / elapsed
			}
		}
		ios = append(ios, io)
	}
	uc.netIO = ios
	uc.lastNet = next
	uc.lastNetAt = now
}

// ServerStatus 组装服务器状态快照（读采样器缓存 + 实时采集低开销指标）
func (uc *Usecase) ServerStatus(ctx context.Context) (*ServerStatus, error) {
	st := &ServerStatus{Time: time.Now().Unix()}
	st.Host = uc.cachedHost()

	uc.mu.RLock()
	st.CPU = CPUInfo{Percent: uc.cpuPct, PerCore: append([]float64(nil), uc.cpuPerCPU...)}
	st.Net = append([]NetIO(nil), uc.netIO...)
	uc.mu.RUnlock()

	// CPU 型号与逻辑核数（变化频率低，随请求采集开销可忽略）
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		st.CPU.ModelName = infos[0].ModelName
		st.CPU.Cores, _ = cpu.Counts(true)
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, ErrCollectFailed
	}
	sm, err := mem.SwapMemory()
	if err != nil {
		return nil, ErrCollectFailed
	}
	st.Memory = MemInfo{
		Total: vm.Total, Used: vm.Used, Available: vm.Available, UsedPercent: vm.UsedPercent,
		SwapTotal: sm.Total, SwapUsed: sm.Used, SwapPercent: sm.UsedPercent,
	}

	st.Disks = uc.disks()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	var gc debug.GCStats
	debug.ReadGCStats(&gc)
	st.Go = GoRuntime{
		Version: runtime.Version(), Goroutines: runtime.NumGoroutine(),
		HeapAlloc: ms.HeapAlloc, SysMemory: ms.Sys,
		GCCount: gc.NumGC, GCPauseMs: float64(gc.PauseTotal.Microseconds()) / 1000,
		ProcessStart: procStart,
	}
	return st, nil
}

// cachedHost 主机静态信息（缓存 hostCacheTTL；采集失败沿用旧值兜底）
func (uc *Usecase) cachedHost() *HostInfo {
	uc.mu.RLock()
	if uc.host != nil && time.Since(uc.hostAt) < hostCacheTTL {
		h := *uc.host
		uc.mu.RUnlock()
		return &h
	}
	uc.mu.RUnlock()

	info, err := host.Info()
	if err != nil {
		logger.Warn("monitor: collect host failed", zap.Error(err))
		uc.mu.RLock()
		h := uc.host
		uc.mu.RUnlock()
		return h
	}
	h := &HostInfo{
		Hostname: info.Hostname, OS: info.OS, Platform: info.Platform,
		PlatformVersion: info.PlatformVersion, KernelArch: info.KernelArch,
		KernelVersion: info.KernelVersion, BootTime: info.BootTime,
	}
	uc.mu.Lock()
	uc.host, uc.hostAt = h, time.Now()
	uc.mu.Unlock()
	return h
}

// disks 物理分区用量（逐分区容错：无权限或不可用的挂载点跳过）
func (uc *Usecase) disks() []DiskInfo {
	parts, err := disk.Partitions(false)
	if err != nil {
		logger.Warn("monitor: list partitions failed", zap.Error(err))
		return nil
	}
	disks := make([]DiskInfo, 0, len(parts))
	for _, p := range parts {
		if diskFstypeSkip[p.Fstype] {
			continue
		}
		// macOS firmlink 子卷（/System/Volumes/*）与根分区同属一个 APFS 容器，只展示根
		if strings.HasPrefix(p.Mountpoint, "/System/Volumes/") {
			continue
		}
		u, err := disk.Usage(p.Mountpoint)
		if err != nil || u.Total == 0 {
			continue
		}
		disks = append(disks, DiskInfo{
			Device: p.Device, Mount: p.Mountpoint, Fstype: p.Fstype,
			Total: u.Total, Used: u.Used, Free: u.Free, UsedPercent: u.UsedPercent,
		})
	}
	return disks
}
