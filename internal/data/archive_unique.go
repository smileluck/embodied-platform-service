// 软删行唯一索引归档：删除（软删留痕）后改写唯一业务列，释放唯一索引供同值重建。
package data

import (
	"strconv"

	"gorm.io/gorm"
)

// archiveUniqueKeep 归档时保留原值前缀长度（列宽 64，后缀 "#"+主键 最长约 11 字符）
const archiveUniqueKeep = 56

// ArchiveUniqueColumns 在软删后把指定唯一业务列改写为 "<前56字符>#<id>"。
// 背景：模型为软删留痕设计，但业务列上有硬唯一索引——软删行会永久占用
// username/code 等值，导致「删除后重建同名/同编码」必然撞唯一索引失败。
// 改写只影响软删行（所有业务查询均已过滤软删），历史行仍可追溯原值含义（保留前缀）。
// 列名全部来自代码内常量（非用户输入），无注入面。
func ArchiveUniqueColumns(tx *gorm.DB, table string, id uint, cols ...string) error {
	keep := strconv.Itoa(archiveUniqueKeep)
	for _, col := range cols {
		stmt := "UPDATE " + table + " SET " + col +
			" = CONCAT(SUBSTRING(" + col + ", 1, " + keep + "), '#', ?) WHERE id = ?"
		if err := tx.Exec(stmt, id, id).Error; err != nil {
			return err
		}
	}
	return nil
}
