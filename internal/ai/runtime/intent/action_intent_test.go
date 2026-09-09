package intent

import "testing"

func TestIsExplicitHandoffRequestHonorsNegation(t *testing.T) {
	tests := []struct {
		message string
		want    bool
	}{
		{message: "请马上转人工", want: true},
		{message: "我需要人工客服协助", want: true},
		{message: "人工", want: true},
		{message: "请告诉我标准复位步骤，暂时不要转人工。", want: false},
		{message: "请先按产品知识给出下一步安全排查，不要自动转人工。", want: false},
		{message: "先别直接联系人工，我想再复测一次", want: false},
		{message: "不用联系人工，我先继续排查", want: false},
		{message: "人工客服怎么收费", want: false},
		{message: "我需要人工处理", want: true},
	}
	for _, tt := range tests {
		if got := IsExplicitHandoffRequest(tt.message); got != tt.want {
			t.Errorf("IsExplicitHandoffRequest(%q) = %v, want %v", tt.message, got, tt.want)
		}
	}
}

func TestIsExplicitTicketRequestHonorsNegation(t *testing.T) {
	tests := []struct {
		message string
		want    bool
	}{
		{message: "请帮我创建工单", want: true},
		{message: "我要报障", want: true},
		{message: "工单", want: true},
		{message: "暂时不要创建工单，我先继续检查", want: false},
		{message: "不要自动创建工单，先给我排查步骤", want: false},
		{message: "先别直接建单", want: false},
		{message: "不用报障", want: false},
	}
	for _, tt := range tests {
		if got := IsExplicitTicketRequest(tt.message); got != tt.want {
			t.Errorf("IsExplicitTicketRequest(%q) = %v, want %v", tt.message, got, tt.want)
		}
	}
}

func TestTicketRequestIssueDetailRemovesActionOnlyText(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{message: "请帮我创建工单", want: ""},
		{message: "创建工单可以吗？", want: ""},
		{message: "请帮我创建工单，设备红灯并显示故障码 E42", want: "设备红灯并显示故障码e42"},
		{message: "设备无法启动，请帮我登记工单", want: "设备无法启动"},
	}
	for _, tt := range tests {
		if got := TicketRequestIssueDetail(tt.message); got != tt.want {
			t.Errorf("TicketRequestIssueDetail(%q) = %q, want %q", tt.message, got, tt.want)
		}
	}
}
