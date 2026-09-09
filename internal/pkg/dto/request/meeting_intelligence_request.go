package request

type MeetingTranscriptIngestRequest struct {
	Provider        string  `json:"provider"`
	ProviderEventID string  `json:"providerEventId"`
	ParticipantID   string  `json:"participantId"`
	SpeakerName     string  `json:"speakerName"`
	Language        string  `json:"language"`
	Text            string  `json:"text"`
	IsFinal         bool    `json:"isFinal"`
	StartedAtMS     int64   `json:"startedAtMs"`
	EndedAtMS       int64   `json:"endedAtMs"`
	Confidence      float64 `json:"confidence"`
}

type MeetingARBoundsRequest struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type MeetingARAnnotationCreateRequest struct {
	FrameAssetID        int64                  `json:"frameAssetId"`
	DetectionProvider   string                 `json:"detectionProvider"`
	ExternalDetectionID string                 `json:"externalDetectionId"`
	Label               string                 `json:"label"`
	PartCode            string                 `json:"partCode"`
	Confidence          float64                `json:"confidence"`
	Bounds              MeetingARBoundsRequest `json:"bounds"`
	Color               string                 `json:"color"`
	Note                string                 `json:"note"`
	MetadataJSON        string                 `json:"metadataJson"`
}
