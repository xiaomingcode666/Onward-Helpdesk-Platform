package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func stubVoiceTicketTitleLLM(t *testing.T, fn voiceTicketTitleGenerator) {
	t.Helper()
	previous := generateVoiceTicketTitleLLM
	generateVoiceTicketTitleLLM = fn
	t.Cleanup(func() { generateVoiceTicketTitleLLM = previous })
}

func TestBuildVoiceTicketTitleUsesLLM(t *testing.T) {
	transcript := "你好，我家的打印机今天早上突然无法开机，指示灯一直闪红色。"
	var gotUser string
	stubVoiceTicketTitleLLM(t, func(_ context.Context, _, userPrompt string) (string, error) {
		gotUser = userPrompt
		return "打印机无法开机", nil
	})
	if got := buildVoiceTicketTitle(context.Background(), transcript); got != "打印机无法开机" {
		t.Fatalf("title = %q", got)
	}
	if !strings.Contains(gotUser, transcript) {
		t.Fatalf("user prompt %q does not contain the transcript", gotUser)
	}
}

func TestBuildVoiceTicketTitleCleansOutput(t *testing.T) {
	stubVoiceTicketTitleLLM(t, func(context.Context, string, string) (string, error) {
		return "标题：「打印机无法开机」\n解释：客户反映打印机故障", nil
	})
	if got := buildVoiceTicketTitle(context.Background(), "打印机坏了"); got != "打印机无法开机" {
		t.Fatalf("title = %q", got)
	}
}

func TestBuildVoiceTicketTitleFallsBackOnError(t *testing.T) {
	stubVoiceTicketTitleLLM(t, func(context.Context, string, string) (string, error) {
		return "", errors.New("llm down")
	})
	transcript := strings.Repeat("打印机", 5) + "\n" + strings.Repeat("无法开机", 7)
	want := string([]rune(strings.ReplaceAll(transcript, "\n", " "))[:30])
	if got := buildVoiceTicketTitle(context.Background(), transcript); got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestBuildVoiceTicketTitleFallsBackOnBlankOutput(t *testing.T) {
	stubVoiceTicketTitleLLM(t, func(context.Context, string, string) (string, error) {
		return " \"\" \n", nil
	})
	if got := buildVoiceTicketTitle(context.Background(), "打印机无法开机"); got != "打印机无法开机" {
		t.Fatalf("title = %q", got)
	}
}

func TestBuildVoiceTicketTitleTruncates(t *testing.T) {
	stubVoiceTicketTitleLLM(t, func(context.Context, string, string) (string, error) {
		return strings.Repeat("长", 300), nil
	})
	if got := buildVoiceTicketTitle(context.Background(), "打印机无法开机"); utf8.RuneCountInString(got) != 200 {
		t.Fatalf("title length = %d, want 200", utf8.RuneCountInString(got))
	}
}
