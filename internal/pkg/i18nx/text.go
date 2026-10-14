package i18nx

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Params 是 Text 的命名参数。值可以是字符串、数字、布尔值或嵌套的 Text。
type Params map[string]any

// Text 是与语言无关的系统文本：保存资源键和参数，在展示时按语言渲染。
// 资源中的占位符使用 {name} 形式；用户录入的内容作为参数原样插入。
type Text struct {
	Key    string `json:"key"`
	Params Params `json:"params,omitempty"`
}

func NewText(key string, params Params) Text {
	return Text{Key: key, Params: params}
}

func (t Text) IsZero() bool {
	return t.Key == ""
}

// Render 按指定语言渲染文本；缺失的翻译回退到默认语言，再回退到资源键。
func (t Text) Render(locale string) string {
	return t.render(NormalizeLocale(locale), lookup)
}

func (t Text) render(locale string, lookupFn func(locale, key string) string) string {
	if t.IsZero() {
		return ""
	}
	format := lookupFn(locale, t.Key)
	if len(t.Params) == 0 || !strings.Contains(format, "{") {
		return format
	}
	var builder strings.Builder
	for {
		start := strings.IndexByte(format, '{')
		if start < 0 {
			builder.WriteString(format)
			break
		}
		end := strings.IndexByte(format[start:], '}')
		if end < 0 {
			builder.WriteString(format)
			break
		}
		end += start
		name := format[start+1 : end]
		builder.WriteString(format[:start])
		if value, ok := t.Params[name]; ok {
			builder.WriteString(renderParam(value, locale, lookupFn))
		} else {
			builder.WriteString(format[start : end+1])
		}
		format = format[end+1:]
	}
	return builder.String()
}

func renderParam(value any, locale string, lookupFn func(locale, key string) string) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case Text:
		return typed.render(locale, lookupFn)
	case *Text:
		if typed == nil {
			return ""
		}
		return typed.render(locale, lookupFn)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	default:
		return fmt.Sprint(typed)
	}
}

func (t *Text) UnmarshalJSON(data []byte) error {
	var raw struct {
		Key    string         `json:"key"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.Key = raw.Key
	t.Params = nil
	if len(raw.Params) > 0 {
		t.Params = make(Params, len(raw.Params))
		for name, value := range raw.Params {
			t.Params[name] = decodeNestedText(value)
		}
	}
	return nil
}

// decodeNestedText 把 JSON 解码后的 {"key": ..., "params": ...} 对象还原为嵌套 Text。
func decodeNestedText(value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	key, ok := object["key"].(string)
	if !ok || key == "" {
		return value
	}
	for field := range object {
		if field != "key" && field != "params" {
			return value
		}
	}
	nested := Text{Key: key}
	if params, ok := object["params"].(map[string]any); ok && len(params) > 0 {
		nested.Params = make(Params, len(params))
		for name, item := range params {
			nested.Params[name] = decodeNestedText(item)
		}
	}
	return nested
}

// Value 将 Text 以 JSON 字符串写入数据库；零值写入空字符串。
func (t Text) Value() (driver.Value, error) {
	if t.IsZero() {
		return "", nil
	}
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (t *Text) Scan(src any) error {
	var data []byte
	switch typed := src.(type) {
	case nil:
		*t = Text{}
		return nil
	case string:
		data = []byte(typed)
	case []byte:
		data = typed
	default:
		return fmt.Errorf("i18nx.Text: unsupported scan type %T", src)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		*t = Text{}
		return nil
	}
	return json.Unmarshal(data, t)
}

// Equal 判断两个文本的资源键和参数是否相同。
func (t Text) Equal(other Text) bool {
	if t.Key != other.Key {
		return false
	}
	if len(t.Params) == 0 && len(other.Params) == 0 {
		return true
	}
	left, leftErr := json.Marshal(t.Params)
	right, rightErr := json.Marshal(other.Params)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}

// MatchesAnyLocale 判断 value 是否等于该文本在任一受支持语言下的渲染结果，
// 用于识别用户提交的表单是否仍保留系统生成的名称。
func (t Text) MatchesAnyLocale(value string) bool {
	if t.IsZero() {
		return false
	}
	value = strings.TrimSpace(value)
	for _, locale := range SupportedLocales {
		if t.Render(locale) == value {
			return true
		}
	}
	return false
}

// RenderValue 按指定语言渲染单个参数值：Text 渲染为当前语言，其他值转为字符串。
func RenderValue(value any, locale string) string {
	return renderParam(value, NormalizeLocale(locale), lookup)
}

// UnmarshalJSON 解码参数，并把 {"key": ..., "params": ...} 形式的值还原为嵌套 Text。
func (p *Params) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		*p = nil
		return nil
	}
	decoded := make(Params, len(raw))
	for name, value := range raw {
		decoded[name] = decodeNestedText(value)
	}
	*p = decoded
	return nil
}

// Value 将参数以 JSON 字符串写入数据库；空参数写入空字符串。
func (p Params) Value() (driver.Value, error) {
	if len(p) == 0 {
		return "", nil
	}
	data, err := json.Marshal(map[string]any(p))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (p *Params) Scan(src any) error {
	var data []byte
	switch typed := src.(type) {
	case nil:
		*p = nil
		return nil
	case string:
		data = []byte(typed)
	case []byte:
		data = typed
	default:
		return fmt.Errorf("i18nx.Params: unsupported scan type %T", src)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		*p = nil
		return nil
	}
	return json.Unmarshal(data, p)
}
