package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestConversationMessageTranslationCachesByMessageAndLanguage(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Conversation{},
		&models.Message{},
		&models.ConversationMessageTranslation{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	conversation := models.Conversation{
		TenantID:     4201,
		CustomerName: "Translation Customer",
		Status:       enums.IMConversationStatusActive,
		LastActiveAt: now,
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "translation-message-1",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "The hydraulic pressure drops after startup.",
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	chatCalls := 0
	originalChat := conversationTranslationChat
	conversationTranslationChat = func(_ context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
		chatCalls++
		if !strings.Contains(systemPrompt, "Simplified Chinese (zh-CN)") {
			t.Fatalf("translation prompt does not name the target language: %q", systemPrompt)
		}
		if chatCalls == 2 && !strings.Contains(systemPrompt, "previous result used the wrong language") {
			t.Fatalf("translation retry is not strict enough: %q", systemPrompt)
		}
		if userPrompt != `"The hydraulic pressure drops after startup."` {
			t.Fatalf("translation source = %q", userPrompt)
		}
		if chatCalls == 1 {
			return &ai.ChatCompletionResult{
				Content:          "The hydraulic pressure drops after startup.",
				ModelName:        "translation-test-model",
				PromptTokens:     10,
				CompletionTokens: 6,
			}, nil
		}
		return &ai.ChatCompletionResult{
			Content:          "液压压力在启动后下降。",
			ModelName:        "translation-test-model",
			PromptTokens:     12,
			CompletionTokens: 8,
		}, nil
	}
	t.Cleanup(func() {
		conversationTranslationChat = originalChat
	})

	operator := &dto.AuthPrincipal{TenantID: 4201, UserID: 7, Username: "translator"}
	req := request.TranslateConversationMessageRequest{MessageID: message.ID, TargetLanguage: "zh-CN"}
	first, err := ConversationTranslationService.Translate(context.Background(), 4201, conversation.ID, req, operator)
	if err != nil {
		t.Fatalf("first Translate: %v", err)
	}
	if first.Cached || first.Translation.SourceLanguage != "en" || first.Translation.TranslatedText != "液压压力在启动后下降。" {
		t.Fatalf("first translation = %+v", first)
	}

	second, err := ConversationTranslationService.Translate(context.Background(), 4201, conversation.ID, req, operator)
	if err != nil {
		t.Fatalf("cached Translate: %v", err)
	}
	if !second.Cached || second.Translation.ID != first.Translation.ID || chatCalls != 2 {
		t.Fatalf("cache miss: first=%+v second=%+v chatCalls=%d", first, second, chatCalls)
	}

	otherTenantOperator := &dto.AuthPrincipal{TenantID: 4202, UserID: 8, Username: "other-tenant"}
	if _, err := ConversationTranslationService.Translate(context.Background(), 4202, conversation.ID, req, otherTenantOperator); err == nil {
		t.Fatal("cross-tenant translation should fail")
	}
}

func TestConversationTranslationFallsBackFromTenantQuotaToPlatformCredential(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Conversation{},
		&models.Message{},
		&models.ConversationMessageTranslation{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	conversation := models.Conversation{
		TenantID:     4301,
		CustomerName: "Fallback Customer",
		Status:       enums.IMConversationStatusActive,
		LastActiveAt: now,
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "translation-fallback-message-1",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "The device is ready for inspection.",
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	originalChat := conversationTranslationChat
	chatCalls := 0
	conversationTranslationChat = func(ctx context.Context, _, _ string) (*ai.ChatCompletionResult, error) {
		chatCalls++
		scope := ai.CapabilityScopeFromContext(ctx)
		if chatCalls == 1 {
			if len(scope.CredentialChain) != 1 || scope.CredentialChain[0] != dsl.ModelCredentialScopeTenantDefault {
				t.Fatalf("first translation credential scope = %#v", scope.CredentialChain)
			}
			return nil, errors.New("tenant translation key quota exhausted")
		}
		if len(scope.CredentialChain) != 1 || scope.CredentialChain[0] != ai.ModelCredentialScopePlatformTranslation {
			t.Fatalf("fallback translation credential scope = %#v", scope.CredentialChain)
		}
		return &ai.ChatCompletionResult{
			Content:          "设备已准备好进行检查。",
			ModelName:        "platform-translation-test-model",
			PromptTokens:     8,
			CompletionTokens: 6,
		}, nil
	}
	t.Cleanup(func() { conversationTranslationChat = originalChat })

	result, err := ConversationTranslationService.Translate(
		context.Background(),
		conversation.TenantID,
		conversation.ID,
		request.TranslateConversationMessageRequest{MessageID: message.ID, TargetLanguage: "zh-CN"},
		&dto.AuthPrincipal{TenantID: conversation.TenantID, UserID: 17, Username: "translator"},
	)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if result == nil || result.Translation == nil || result.Translation.TranslatedText != "设备已准备好进行检查。" || chatCalls != 2 {
		t.Fatalf("fallback translation = %+v, chatCalls=%d", result, chatCalls)
	}
}

func TestConversationTranslationLanguageDetectionAndValidation(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		target string
		want   string
	}{
		{name: "english", text: "Can I safely restart the device?", want: "en"},
		{name: "chinese with fault code", text: "设备显示 RHD-FLOW-ALPHA-7742，可以继续上电吗？", want: "zh-CN"},
		{name: "japanese", text: "装置を再起動しても安全ですか", want: "ja"},
		{name: "korean", text: "장치를 다시 시작해도 안전합니까", want: "ko"},
		{name: "unknown latin", text: "Reiniciar controlador", want: "und-Latn"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := detectConversationMessageLanguage(test.text); got != test.want {
				t.Fatalf("detectConversationMessageLanguage(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}

	if translationOutputMatchesTarget("en", "zh-CN", "The device should remain powered off.") {
		t.Fatal("English output must not pass Simplified Chinese target validation")
	}
	if !translationOutputMatchesTarget("en", "zh-CN", "设备应保持断电。") {
		t.Fatal("Chinese output should pass Simplified Chinese target validation")
	}
	if translationOutputMatchesTarget("zh-CN", "en", "设备应保持断电。") {
		t.Fatal("Chinese output must not pass English target validation")
	}
	if !translationOutputMatchesTarget("zh-CN", "en", "Keep the device powered off.") {
		t.Fatal("English output should pass English target validation")
	}
}
