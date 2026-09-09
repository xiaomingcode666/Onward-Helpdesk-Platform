package response

type MeetingARFrameResponse struct {
	AssetID int64  `json:"assetId"`
	URL     string `json:"url"`
}

type MeetingTranscriptSegmentResponse struct {
	ID                  string  `json:"id"`
	MeetingID           string  `json:"meetingId"`
	ParticipantID       string  `json:"participantId"`
	SpeakerName         string  `json:"speakerName"`
	Provider            string  `json:"provider"`
	ProviderEventID     string  `json:"providerEventId"`
	IngestSource        string  `json:"ingestSource"`
	Language            string  `json:"language"`
	Text                string  `json:"text"`
	IsFinal             bool    `json:"isFinal"`
	StartedAtMS         int64   `json:"startedAtMs"`
	EndedAtMS           int64   `json:"endedAtMs"`
	Confidence          float64 `json:"confidence"`
	TranslatedLanguage  string  `json:"translatedLanguage"`
	TranslatedText      string  `json:"translatedText"`
	TranslationProvider string  `json:"translationProvider"`
	TranslationStatus   string  `json:"translationStatus"`
	TranslationError    string  `json:"translationError,omitempty"`
	CreatedAt           string  `json:"createdAt"`
}

type MeetingARBoundsResponse struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type MeetingARAnnotationResponse struct {
	ID                  string                  `json:"id"`
	MeetingID           string                  `json:"meetingId"`
	TicketID            string                  `json:"ticketId"`
	FrameAssetID        int64                   `json:"frameAssetId"`
	FrameURL            string                  `json:"frameUrl,omitempty"`
	DetectionProvider   string                  `json:"detectionProvider"`
	ExternalDetectionID string                  `json:"externalDetectionId"`
	Label               string                  `json:"label"`
	PartCode            string                  `json:"partCode"`
	Confidence          float64                 `json:"confidence"`
	Bounds              MeetingARBoundsResponse `json:"bounds"`
	Color               string                  `json:"color"`
	Note                string                  `json:"note"`
	CreatedBy           int64                   `json:"createdBy"`
	MetadataJSON        string                  `json:"metadataJson"`
	CreatedAt           string                  `json:"createdAt"`
}
