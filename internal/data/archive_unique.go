// 软删行唯一索引归档：删除（软删留痕）后改写唯一业务列，释放唯一索引供同值重建。
package data

import (
	"strconv"

	"gorm.io/gorm"
)

// ArchiveUniqueColumns 在软删后把指定唯一业务列改写为 "<前缀>#<id>"。
// 背景：模型为软删留痕设计，但业务列上有硬唯一索引——软删行会永久占用
// username/code 等值，导致「删除后重建同名/同编码」必然撞唯一索引失败。
// 改写只影响软删行（所有业务查询均已过滤软删），历史行仍可追溯原值含义（保留前缀）。
// sizes 为列名 -> 列宽（字符数，与 gorm size 一致）：截断长度 = 列宽 - 后缀长度，
// 逐列计算避免固定截断在窄列（如 dict_items.label 20）上拼出超长值报错 1406。
// 列名全部来自代码内常量（非用户输入），无注入面。
func ArchiveUniqueColumns(tx *gorm.DB, table string, id uint, sizes map[string]int) error {
	suffix := "#" + strconv.FormatUint(uint64(id), 10)
	for col, size := range sizes {
		keep := size - len(suffix)
		if keep < 1 {
			keep = 1
		}
		stmt := "UPDATE " + table + " SET " + col +
			" = CONCAT(SUBSTRING(" + col + ", 1, " + strconv.Itoa(keep) + "), ?) WHERE id = ?"
		if err := tx.Exec(stmt, suffix, id).Error; err != nil {
			return err
		}
	}
	return nil
}
