package tool

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// validateInput 用工具声明的 JSON Schema 校验 LLM 产出的参数（可信代码校验）。
// 禁止把 LLM 输出直接透传给 service（docs/CONVENTIONS-backend.md §3）。
func validateInput(spec Spec, input json.RawMessage) error {
	if len(spec.InputSchema) == 0 {
		return nil // 无参工具
	}
	loader := gojsonschema.NewBytesLoader(spec.InputSchema)
	doc := gojsonschema.NewBytesLoader(input)
	res, err := gojsonschema.Validate(loader, doc)
	if err != nil {
		return fmt.Errorf("参数校验失败: %w", err)
	}
	if !res.Valid() {
		msgs := ""
		for i, e := range res.Errors() {
			if i > 0 {
				msgs += "; "
			}
			msgs += e.Description()
		}
		return fmt.Errorf("参数不合法: %s", msgs)
	}
	return nil
}

// DecodeInput 把 JSON 参数解码到结构体，供工具实现使用。
func DecodeInput[T any](input json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(input, &v); err != nil {
		return v, fmt.Errorf("参数解析失败: %w", err)
	}
	return v, nil
}
