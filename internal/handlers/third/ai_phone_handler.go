package third

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"

	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

const aiPhoneMaxBodyBytes = 512 * 1024

type aiPhoneTicketRequest struct {
	SchemaVersion      int              `json:"schema_version"`
	Provider           string           `json:"provider"`
	CallID             string           `json:"call_id"`
	IntentID           string           `json:"intent_id"`
	Language           string           `json:"language"`
	WorkLanguage       string           `json:"work_language"`
	Title              string           `json:"title"`
	Description        string           `json:"description"`
	ProjectKey         string           `json:"project_key"`
	TicketType         string           `json:"ticket_type"`
	CallerName         string           `json:"caller_name"`
	CallerPhone        string           `json:"caller_phone"`
	MissingFields      []string         `json:"missing_fields"`
	RecordingRef       string           `json:"recording_ref"`
	TranslatedSummary  string           `json:"translated_summary"`
	TranscriptSegments []map[string]any `json:"transcript_segments"`
	FieldEvidence      map[string]any   `json:"field_evidence"`
	PriorityCode       string           `json:"priority_code"`
	ProductID          int64            `json:"product_id"`
	ProductModelID     int64            `json:"product_model_id"`
	ProductModuleID    int64            `json:"product_module_id"`
	DeviceID           int64            `json:"device_id"`
	ServiceCodeID      int64            `json:"service_code_id"`
	ServiceRegion      string           `json:"service_region"`
	FaultCode          string           `json:"fault_code"`
	SymptomSummary     string           `json:"symptom_summary"`
}

// AIPhonePostTicket is the service-to-service ticket boundary for the Python
// voice agent. The handler never trusts a tenant ID from the JSON body.
func AIPhonePostTicket(ctx *gin.Context) {
	if !validAIPhoneServiceToken(ctx.GetHeader("Authorization")) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	lineID := strings.TrimSpace(ctx.GetHeader("X-AI-Phone-Line-ID"))
	tenantID, ok := aiPhoneTenantForLine(lineID)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "phone line is not bound to a tenant"})
		return
	}

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, aiPhoneMaxBodyBytes)
	var input aiPhoneTicketRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := input.validate(); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	expectedIdempotencyKey := input.Provider + ":" + input.CallID + ":" + input.IntentID
	if provided := strings.TrimSpace(ctx.GetHeader("Idempotency-Key")); provided != "" && provided != expectedIdempotencyKey {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "idempotency key does not match call identity"})
		return
	}
	if strings.TrimSpace(ctx.GetHeader("Idempotency-Key")) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key is required"})
		return
	}

	operator := services.NewAIPhoneOperator(tenantID)
	item, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		TicketIntakeInput: dto.TicketIntakeInput{
			SourceRecordID: input.Provider + ":" + input.CallID + ":" + input.IntentID,
			ProjectKey:     input.ProjectKey,
			TicketType:     input.TicketType,
			CallerName:     input.CallerName,
			CallerPhone:    input.CallerPhone,
		},
		IdempotencyKey:  expectedIdempotencyKey,
		Title:           input.Title,
		Description:     input.Description,
		Source:          string(enums.TicketSourceAIPhone),
		Channel:         "phone",
		PriorityCode:    input.PriorityCode,
		TenantID:        tenantID,
		ProductID:       input.ProductID,
		ProductModelID:  input.ProductModelID,
		ProductModuleID: input.ProductModuleID,
		DeviceID:        input.DeviceID,
		ServiceCodeID:   input.ServiceCodeID,
		ServiceRegion:   input.ServiceRegion,
		FaultCode:       input.FaultCode,
		SymptomSummary:  input.SymptomSummary,
	}, operator)
	if err != nil {
		if errors.Is(err, services.ErrTicketIdempotencyConflict) {
			ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "idempotency key payload conflict"})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "ticket creation failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success":   true,
		"ticket_id": item.ID,
		"ticket_no": item.TicketNo,
		"status":    item.Status,
	})
}

func (r aiPhoneTicketRequest) validate() error {
	if r.SchemaVersion != 1 {
		return errors.New("schema_version must be 1")
	}
	for name, value := range map[string]string{
		"provider": r.Provider, "call_id": r.CallID, "intent_id": r.IntentID,
		"language": r.Language, "work_language": r.WorkLanguage,
		"title": r.Title, "description": r.Description,
		"project_key": r.ProjectKey, "ticket_type": r.TicketType,
	} {
		if strings.TrimSpace(value) == "" {
			return errors.New(name + " is required")
		}
	}
	if len([]rune(r.Provider+":"+r.CallID+":"+r.IntentID)) > 160 {
		return errors.New("call identity is too long")
	}
	for name, value := range map[string]string{
		"language": r.Language, "work_language": r.WorkLanguage, "title": r.Title,
		"description": r.Description, "project_key": r.ProjectKey, "ticket_type": r.TicketType,
		"caller_name": r.CallerName, "caller_phone": r.CallerPhone, "recording_ref": r.RecordingRef,
		"service_region": r.ServiceRegion, "fault_code": r.FaultCode, "symptom_summary": r.SymptomSummary,
	} {
		if len([]rune(value)) > 20000 || strings.ContainsRune(value, '\x00') {
			return errors.New(name + " is too long or contains invalid characters")
		}
	}
	if len(r.MissingFields) > 32 || len(r.TranscriptSegments) > 2000 || len(r.FieldEvidence) > 128 {
		return errors.New("request contains too many structured fields")
	}
	return nil
}

func aiPhoneServiceAccountID() int64 {
	return services.AIPhoneServiceAccountID()
}

func validAIPhoneServiceToken(header string) bool {
	configured := strings.TrimSpace(os.Getenv("AI_PHONE_SERVICE_TOKEN"))
	if len(configured) < 32 {
		return false
	}
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	provided := strings.TrimSpace(parts[1])
	return len(provided) == len(configured) && subtle.ConstantTimeCompare([]byte(provided), []byte(configured)) == 1
}

func aiPhoneTenantForLine(lineID string) (int64, bool) {
	return services.AIPhoneTenantForLine(lineID)
}
