// 软删除与唯一编码冲突的通用处理（与 embodied-platform 的 data.TombstoneCode 同构——
// 两仓库同名 module 但互不依赖，各自维护一份；改格式时两侧同步）。
// 软删除的行仍占用单列唯一索引，若不改写编码，删除后同编码新增必然触发唯一冲突。
// 方案：软删除时将编码改写为墓碑编码（原编码_del_<id>_<unix时间戳>），释放唯一索引。
package data

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// codeColumnSize 编码列宽度（各 PO 的 code/name 均为 size:64）
const codeColumnSize = 64

// TombstoneCode 生成软删除墓碑编码：原编码_del_<id>_<unix时间戳>，
// 总长截断到列宽（64 字符，按 rune 截断——编码/名称可为多字节字符，字节截断会拆坏 UTF-8）。
func TombstoneCode(code string, id uint, now time.Time) string {
	suffix := fmt.Sprintf("_del_%d_%d", id, now.Unix())
	if utf8.RuneCountInString(code)+len(suffix) > codeColumnSize {
		code = string([]rune(code)[:codeColumnSize-len(suffix)])
	}
	return code + suffix
}

// IsUniqueViolation 各数据库唯一约束冲突文案判断：
// MySQL Error 1062、Postgres 23505（duplicate key value）、SQLite UNIQUE constraint failed。
// 供各仓储 mapErr 统一接入（避免逐 repo 复制文案匹配）
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "UNIQUE constraint failed")
}
