package i18nx

import "testing"

func TestLocalizeCustomerConversationSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		locale           string
		summary          string
		hasDeviceConcept bool
		want             string
	}{
		{
			name:             "knowledge welcome follows requested language",
			locale:           LocaleEnUS,
			summary:          "您好，我是企业知识服务助手。请直接告诉我你想了解的问题。",
			hasDeviceConcept: false,
			want:             "Hello, I'm your company's knowledge assistant. Tell me what you'd like to know.",
		},
		{
			name:             "feedback label is translated and comment is preserved",
			locale:           LocaleEnUS,
			summary:          "客户已提交服务评价：5/5 · ODT acceptance passed.",
			hasDeviceConcept: false,
			want:             "Customer submitted service feedback: 5/5 · ODT acceptance passed.",
		},
		{
			name:             "video summary keeps room and localizes duration",
			locale:           LocaleEnUS,
			summary:          "视频协作已结束，会议室：rhd-test，时长：9 秒",
			hasDeviceConcept: false,
			want:             "Video collaboration ended. Room: rhd-test. Duration: 9 sec",
		},
		{
			name:             "legacy product queue copy becomes support copy for productless tenant",
			locale:           LocaleEnUS,
			summary:          "你的请求已进入产品维修组待认领，组内工程师可以查看并接单；若当前无人值守，排班恢复后会继续处理。",
			hasDeviceConcept: false,
			want:             "Your request is now visible to the assigned technical support team for pickup. If nobody is on duty, it will be handled when the next shift starts.",
		},
		{
			name:             "product tenant keeps product team meaning",
			locale:           LocaleEnUS,
			summary:          "你的请求已进入产品维修组待认领，组内工程师可以查看并接单；若当前无人值守，排班恢复后会继续处理。",
			hasDeviceConcept: true,
			want:             "Your request is now visible to the product repair team for pickup. If nobody is on duty, it will be handled when the next shift starts.",
		},
		{
			name:             "custom business content is unchanged",
			locale:           LocaleEnUS,
			summary:          "客户反馈登录失败，请联系 Alice",
			hasDeviceConcept: false,
			want:             "客户反馈登录失败，请联系 Alice",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LocalizeCustomerConversationSummary(tt.locale, tt.summary, tt.hasDeviceConcept)
			if got != tt.want {
				t.Fatalf("LocalizeCustomerConversationSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
