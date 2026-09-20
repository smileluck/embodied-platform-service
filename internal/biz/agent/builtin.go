// 内置示范工具（只读）：now 返回服务器时间；get_server_status 返回监控快照摘要。
// 均不产生副作用，可作为 function calling 的最小可用闭环示例。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bizmonitor "github.com/smilex/smilex-admin-gin/internal/biz/monitor"
)

// ServerStatusReader 只读服务器状态（由 monitor 上下文实现，wire 绑定；未注入时不注册该工具）
type ServerStatusReader interface {
	ServerStatus(ctx context.Context) (*bizmonitor.ServerStatus, error)
}

// emptySchema 无参数工具的 JSON Schema
func emptySchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// newNowTool 服务器当前时间
func newNowTool() Tool {
	return &funcTool{
		def: ToolFunc{
			Name:        "now",
			Description: "获取服务器当前时间（含时区），用于涉及时间/日期的问题",
			Parameters:  emptySchema(),
		},
		exec: func(ctx context.Context, args string) (string, error) {
			if err := simpleJSONArg(args); err != nil {
				return "", err
			}
			return time.Now().Format("2006-01-02 15:04:05 -0700 MST"), nil
		},
	}
}

// newServerStatusTool 服务器状态快照（CPU/内存摘要，只读）
func newServerStatusTool(mon ServerStatusReader) Tool {
	return &funcTool{
		def: ToolFunc{
			Name:        "get_server_status",
			Description: "获取服务器当前状态摘要：主机、CPU 占用率、内存与各分区磁盘占用",
			Parameters:  emptySchema(),
		},
		exec: func(ctx context.Context, args string) (string, error) {
			if err := simpleJSONArg(args); err != nil {
				return "", err
			}
			st, err := mon.ServerStatus(ctx)
			if err != nil {
				return "", fmt.Errorf("采集失败: %w", err)
			}
			summary := map[string]any{
				"hostname":       st.Host.Hostname,
				"os":             st.Host.OS,
				"cpu_percent":    st.CPU.Percent,
				"cpu_cores":      st.CPU.Cores,
				"memory_percent": st.Memory.UsedPercent,
				"disks": func() []map[string]any {
					out := make([]map[string]any, 0, len(st.Disks))
					for _, d := range st.Disks {
						out = append(out, map[string]any{"mount": d.Mount, "percent": d.UsedPercent})
					}
					return out
				}(),
			}
			if st.Host == nil {
				delete(summary, "hostname")
				delete(summary, "os")
			}
			b, err := json.Marshal(summary)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}
}
