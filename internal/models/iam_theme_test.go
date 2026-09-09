package models

import "testing"

func TestNormalizeTenantCustomerTheme(t *testing.T) {
	tests := map[string]string{
		"":                   TenantCustomerThemeDefault,
		"unknown":            TenantCustomerThemeDefault,
		" ODT-INTELLIGENCE ": TenantCustomerThemeODTIntelligence,
		"clinical-calm":      TenantCustomerThemeClinicalCalm,
		"signal-coral":       TenantCustomerThemeSignalCoral,
	}

	for input, expected := range tests {
		if actual := NormalizeTenantCustomerTheme(input); actual != expected {
			t.Fatalf("NormalizeTenantCustomerTheme(%q) = %q, want %q", input, actual, expected)
		}
	}
}
