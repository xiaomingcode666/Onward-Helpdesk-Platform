package utils

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func NormalizeNullableString(value *string) *string {
	if value == nil {
		return nil
	}
	v := strings.TrimSpace(*value)
	if v == "" {
		return nil
	}
	return &v
}

func BuildAuditFields(operator *dto.AuthPrincipal) models.AuditFields {
	now := time.Now()
	fields := models.AuditFields{
		CreatedAt: now,
		UpdatedAt: now,
	}
	if operator != nil {
		fields.CreateUserID = operator.UserID
		fields.CreateUserName = operator.Username
		fields.UpdateUserID = operator.UserID
		fields.UpdateUserName = operator.Username
	}
	return fields
}

func FormatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.DateTime)
}

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateTime)
}

func GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		return "", errorsx.InvalidParamI18n("error.e0175")
	}

	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = charset[int(random[i])%len(charset)]
	}
	return string(buf), nil
}

func JoinInt64s(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, cast.ToString(value))
	}
	return strings.Join(parts, ",")
}

func SplitInt64s(raw string) []int64 {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	ret := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{})
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	return ret
}

func NormalizeStringList(values []string) []string {
	ret := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	return ret
}

func MarshalStringListJSON(values []string) string {
	raw, err := json.Marshal(NormalizeStringList(values))
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func ParseStringListJSON(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return []string{}
	}
	ret := make([]string, 0)
	if err := json.Unmarshal([]byte(value), &ret); err == nil {
		return NormalizeStringList(ret)
	}
	return NormalizeStringList(strings.Split(value, ","))
}

// UUID 生成 UUID v4 字符串
func UUID() string {
	return uuid.New().String()
}

// FormattedTime 格式化为 ISO 时间字符串
func FormattedTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// RandomSuffix 生成指定长度的随机字母数字后缀
func RandomSuffix(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("%x", random)[:length]
	}
	for i := range buf {
		buf[i] = charset[int(random[i])%len(charset)]
	}
	return string(buf)
}
