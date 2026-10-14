package services

import (
	"testing"

	"remotehelpdesk/internal/pkg/i18nx"
)

// 状态码类历史文本改为显示状态名称，其余系统进度渲染回中文与原文逐字相同。
var legacyStatusCodeTemplates = map[string]string{
	"工单主状态：acknowledged → assigned":       "工单主状态：已受理 → 已分配",
	"工单主状态：new → acknowledged；工程师接单并开始处理": "工单主状态：新建 → 已受理；工程师接单并开始处理",
	"状态流转：processing":                     "状态流转：处理中",
}

// 历史系统进度按固定模板识别；用户录入的内容不识别。
func TestLegacyProgressTextRecognizesSystemTemplates(t *testing.T) {
	cases := map[string]string{
		"关联会话状态已恢复为待接入":                       "Linked conversation returned to waiting for pickup",
		"工单随会话自动分配，原因：自动分配":                   "Ticket assigned automatically with the conversation. Reason: automatic assignment",
		"技术支持组成员接管并受理":                        "Taken over and accepted by a technical support team member",
		"支持状态检查：支持正常":                         "Support check: Support available",
		"已确认收到问题":                             "An engineer has taken the case and will follow up.",
		"工单主状态：acknowledged → assigned":       "Case phase: Accepted → Assigned",
		"工单主状态：new → acknowledged；工程师接单并开始处理": "Case phase: New → Accepted · Engineer accepted the case and started work",
		"状态流转：processing":                     "Status changed to In progress",
		"分配工单，原处理人：alice -> bob，原因：客户要求":      "Ticket reassigned from alice to bob. Reason: 客户要求",
		"工单自动分配 -> 等待工程师接单":                   "Ticket dispatched (automatic assignment); waiting for the engineer to accept",
		"视频协作已结束，会议室：room-1，时长：3 分 5 秒":       "Video collaboration ended. Room: room-1. Duration: 3 min 5 s",
		"Created ticket": "Ticket created",
		"供应商处理完成：更换主板":   "Supplier completed the work: 更换主板",
	}
	for content, want := range cases {
		text, ok := legacyProgressText(content)
		if !ok {
			t.Errorf("%q was not recognized", content)
			continue
		}
		if got := text.Render(i18nx.LocaleEnUS); got != want {
			t.Errorf("%q rendered %q, want %q", content, got, want)
		}
		wantZh := content
		if converted, ok := legacyStatusCodeTemplates[content]; ok {
			wantZh = converted
		}
		if content != "Created ticket" {
			if got := text.Render(i18nx.LocaleZhCN); got != wantZh {
				t.Errorf("%q zh-CN render = %q, want %q", content, got, wantZh)
			}
		}
	}
	for _, content := range []string{"客户说设备有异响", "已更换滤芯，请客户观察", ""} {
		if _, ok := legacyProgressText(content); ok {
			t.Errorf("user content %q should not be recognized", content)
		}
	}
}
