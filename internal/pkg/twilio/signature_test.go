package twilio

import "testing"

func TestValidateSignatureMatchesTwilioHMAC(t *testing.T) {
	params := map[string]string{
		"From": "whatsapp:+8613800000000",
		"Body": "hello",
	}
	const token = "auth-token"
	const url = "https://helpdesk.example/api/third/twilio/whatsapp/webhook"
	signature := Sign(token, url, params)
	if signature == "" || !ValidateSignature(token, url, params, signature) {
		t.Fatalf("ValidateSignature() rejected signature %q", signature)
	}
	if ValidateSignature(token, url, params, "invalid") {
		t.Fatal("ValidateSignature() accepted an invalid signature")
	}
}

func TestNormalizeWhatsAppAddress(t *testing.T) {
	for input, want := range map[string]string{
		"whatsapp:+8613800000000":   "+8613800000000",
		"+8613800000000":            "+8613800000000",
		"  whatsapp:8613800000000 ": "8613800000000",
	} {
		if got := NormalizeWhatsAppAddress(input); got != want {
			t.Fatalf("NormalizeWhatsAppAddress(%q) = %q, want %q", input, got, want)
		}
	}
}
