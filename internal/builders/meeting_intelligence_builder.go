package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildMeetingTranscriptList(items []models.MeetingTranscriptSegment) []response.MeetingTranscriptSegmentResponse {
	result := make([]response.MeetingTranscriptSegmentResponse, 0, len(items))
	for i := range items {
		result = append(result, *BuildMeetingTranscript(&items[i]))
	}
	return result
}

func BuildMeetingTranscript(item *models.MeetingTranscriptSegment) *response.MeetingTranscriptSegmentResponse {
	if item == nil {
		return nil
	}
	return &response.MeetingTranscriptSegmentResponse{
		ID: item.ID, MeetingID: item.MeetingID, ParticipantID: item.ParticipantID,
		SpeakerName: item.SpeakerName, Provider: item.Provider, ProviderEventID: item.ProviderEventID, Language: item.Language,
		IngestSource: item.IngestSource, Text: item.Text, IsFinal: item.IsFinal, StartedAtMS: item.StartedAtMS,
		EndedAtMS: item.EndedAtMS, Confidence: item.Confidence,
		TranslatedLanguage: item.TranslatedLanguage, TranslatedText: item.TranslatedText,
		TranslationProvider: item.TranslationProvider, TranslationStatus: item.TranslationStatus,
		TranslationError: item.TranslationError, CreatedAt: utils.FormatTime(item.CreatedAt),
	}
}

func BuildMeetingARAnnotation(item *models.MeetingARAnnotation, frameURL string) *response.MeetingARAnnotationResponse {
	if item == nil {
		return nil
	}
	return &response.MeetingARAnnotationResponse{
		ID: item.ID, MeetingID: item.MeetingID, TicketID: item.TicketID,
		FrameAssetID: item.FrameAssetID, FrameURL: frameURL, DetectionProvider: item.DetectionProvider,
		ExternalDetectionID: item.ExternalDetectionID, Label: item.Label,
		PartCode: item.PartCode, Confidence: item.Confidence,
		Bounds: response.MeetingARBoundsResponse{X: item.X, Y: item.Y, Width: item.Width, Height: item.Height},
		Color:  item.Color, Note: item.Note, CreatedBy: item.CreatedBy,
		MetadataJSON: item.MetadataJSON, CreatedAt: utils.FormatTime(item.CreatedAt),
	}
}

func BuildMeetingARAnnotationList(items []models.MeetingARAnnotation, frameURLs map[int64]string) []response.MeetingARAnnotationResponse {
	result := make([]response.MeetingARAnnotationResponse, 0, len(items))
	for i := range items {
		if item := BuildMeetingARAnnotation(&items[i], frameURLs[items[i].FrameAssetID]); item != nil {
			result = append(result, *item)
		}
	}
	return result
}
