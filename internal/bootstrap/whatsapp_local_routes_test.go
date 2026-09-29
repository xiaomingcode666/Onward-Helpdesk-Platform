package bootstrap

import "testing"

func TestWhatsAppLocalTestEnabledRequiresExplicitTrue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset", want: false},
		{name: "false", value: "false", want: false},
		{name: "true", value: "true", want: true},
		{name: "case insensitive", value: " TRUE ", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RHD_WHATSAPP_LOCAL_TEST", tt.value)
			if got := whatsappLocalTestEnabled(); got != tt.want {
				t.Fatalf("whatsappLocalTestEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
