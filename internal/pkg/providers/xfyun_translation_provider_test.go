package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/config"
)

func TestXfyunTextTranslationProviderSignsAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v2/its" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		for _, header := range []string{"Date", "Digest", "Authorization"} {
			if strings.TrimSpace(req.Header.Get(header)) == "" {
				t.Fatalf("missing %s header", header)
			}
		}
		if !strings.Contains(req.Header.Get("Authorization"), `api_key="translation-key"`) {
			t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
		}
		var body struct {
			Common   map[string]string `json:"common"`
			Business map[string]string `json:"business"`
			Data     map[string]string `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(body.Data["text"])
		if err != nil || string(decoded) != "设备已重新启动" {
			t.Fatalf("encoded text = %q, %v", string(decoded), err)
		}
		if body.Common["app_id"] != "translation-app" || body.Business["from"] != "cn" || body.Business["to"] != "en" {
			t.Fatalf("translation request = %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"message":"success","sid":"its-test","data":{"result":{"from":"cn","to":"en","trans_result":{"src":"设备已重新启动","dst":"The device has restarted."}}}}`))
	}))
	defer server.Close()

	provider := NewXfyunTextTranslationProvider(config.XfyunSpeechConfig{
		AppID: "translation-app", APIKey: "translation-key", TranslationAPISecret: "translation-secret",
		TranslationEndpoint: server.URL + "/v2/its", TranslationTargetLanguage: "en",
	})
	result, err := provider.Translate(context.Background(), TextTranslationRequest{
		Text: "设备已重新启动", SourceLanguage: "zh-CN",
	})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if result.Text != "The device has restarted." || result.SourceLanguage != "cn" || result.TargetLanguage != "en" || result.ProviderEventID != "its-test" {
		t.Fatalf("translation result = %#v", result)
	}
}

func TestXfyunTextTranslationProviderRejectsOversizedText(t *testing.T) {
	provider := NewXfyunTextTranslationProvider(config.XfyunSpeechConfig{
		AppID: "app", APIKey: "key", TranslationAPISecret: "secret",
	})
	_, err := provider.Translate(context.Background(), TextTranslationRequest{Text: strings.Repeat("测", 1500), SourceLanguage: "zh-CN"})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized Translate() error = %v", err)
	}
}

func TestTextTranslationRemainsAvailableWithAliyunSpeech(t *testing.T) {
	initTextTranslation(&config.SpeechConfig{})
	t.Cleanup(func() { initTextTranslation(&config.SpeechConfig{}) })

	initTextTranslation(&config.SpeechConfig{
		Provider: "aliyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID: "translation-app", APIKey: "translation-key", TranslationAPISecret: "translation-secret",
			TranslationEnabled: true,
		},
	})
	provider := CurrentTextTranslationProvider()
	if provider.Name() != "xfyun_translation" || !provider.Configured() {
		t.Fatalf("translation provider = %s configured=%v", provider.Name(), provider.Configured())
	}
}
