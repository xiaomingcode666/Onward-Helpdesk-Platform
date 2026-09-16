package services

import (
	"regexp"
	"strings"
)

// notificationSensitiveBlockedPrefix 用于标识「因敏感信息被拦截」的通知创建失败。
// 队列消费端据此判定为不可重试，避免同一条通知反复写入拦截记录。
const notificationSensitiveBlockedPrefix = "通知内容命中敏感信息规则"

// NotificationSensitiveFinding 描述一次敏感信息命中。
type NotificationSensitiveFinding struct {
	Code  string
	Label string
}

type notificationSensitiveRule struct {
	Code    string
	Label   string
	Pattern *regexp.Regexp
}

// 发送前拦截敏感信息：命中任意一条即阻断本次发送，并把原因写入发送尝试记录。
var notificationSensitiveRules = []notificationSensitiveRule{
	{Code: "id_card_cn", Label: "身份证号", Pattern: regexp.MustCompile(`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[0-9Xx]`)},
	{Code: "bank_card", Label: "银行账号", Pattern: regexp.MustCompile(`\b\d{16,19}\b`)},
	{Code: "phone_cn", Label: "手机号", Pattern: regexp.MustCompile(`(?:\+?86[\s-]?)?1[3-9]\d{9}(?:\b|$)`)},
	{Code: "email", Label: "邮箱地址", Pattern: regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
}

// ScanNotificationSensitiveContent 检查即将发出的通知内容，命中则返回规则信息。
func ScanNotificationSensitiveContent(text string) *NotificationSensitiveFinding {
	value := strings.TrimSpace(text)
	if value == "" {
		return nil
	}
	for _, rule := range notificationSensitiveRules {
		if rule.Pattern.MatchString(value) {
			return &NotificationSensitiveFinding{Code: rule.Code, Label: rule.Label}
		}
	}
	return nil
}

// DescribeNotificationSensitiveFinding 只输出规则名称，避免把敏感原文写入日志与记录。
func DescribeNotificationSensitiveFinding(finding *NotificationSensitiveFinding) string {
	if finding == nil {
		return ""
	}
	return "命中敏感信息规则：" + finding.Label + "（" + finding.Code + "）"
}
