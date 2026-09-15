package logprivacy

import (
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"testing"
)

func TestValueOmitsArbitrarySensitiveContent(t *testing.T) {
	for _, value := range []string{
		`{"name":"张三","nested":[{"email":"customer@example.test","token":"secret"}]}`,
		`{"malformed":"customer@example.test`,
		"电话 13812345678，地址上海市测试路，证件号码110101199001011234",
		"https://user:password@example.test/reset/secret?token=encoded%40secret#private",
		"Y3VzdG9tZXJAZXhhbXBsZS50ZXN0", "<html>private customer record</html>",
		Redacted,
	} {
		if got := Value(value); got != Redacted {
			t.Errorf("untrusted value was not omitted")
		}
	}
	if Value("") != "" || Value(" \n ") != "" {
		t.Fatal("empty input must stay empty")
	}
}

func TestErrorKeepsOnlyTypedCategory(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""}, {errors.New("customer@example.test password=secret"), "operation failed (details redacted)"},
		{fmt.Errorf("private details: %w", &textproto.Error{Code: 550, Msg: "customer@example.test rejected"}), "SMTP 550 (details redacted)"},
		{context.DeadlineExceeded, "operation timed out"}, {context.Canceled, "operation cancelled"},
	}
	for _, test := range tests {
		if got := Error(test.err); got != test.want {
			t.Fatalf("category = %q, want %q", got, test.want)
		}
	}
}
