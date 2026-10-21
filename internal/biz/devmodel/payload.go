// 透传结构大小兜底：物模型 Schema / 数据映射是纯透传平台开放面的结构，
// 嵌套 map（TMSchema 三层 map、Mapping.FieldExtract）无法用 binding tag 约束总规模，
// 统一按 JSON 序列化字节数设上限，超限时拒绝透传。
package devmodel

import (
	"encoding/json"
	"errors"
)

// ErrPayloadTooLarge 提交的透传结构（物模型 Schema / 数据映射）超过字节上限
var ErrPayloadTooLarge = errors.New("payload too large")

// payloadMaxBytes 透传结构 JSON 序列化字节上限
const payloadMaxBytes = 256 * 1024

// checkPayloadSize 序列化后超 payloadMaxBytes 返回 ErrPayloadTooLarge
func checkPayloadSize(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > payloadMaxBytes {
		return ErrPayloadTooLarge
	}
	return nil
}
