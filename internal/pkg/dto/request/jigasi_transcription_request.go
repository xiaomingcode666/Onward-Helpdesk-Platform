package request

type JigasiTranscriptParticipant struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	IdentityName    string `json:"identity_name"`
	IdentityUserID  string `json:"identity_id"`
	IdentityGroupID string `json:"identity_group_id"`
}

type JigasiTranscriptAlternative struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// JigasiTranscriptEventRequest matches Jigasi's
// RemotePublisherTranscriptionHandler payload.
type JigasiTranscriptEventRequest struct {
	Type        string                        `json:"type"`
	RoomName    string                        `json:"room_name"`
	Event       string                        `json:"event"`
	Timestamp   int64                         `json:"timestamp"`
	Participant JigasiTranscriptParticipant   `json:"participant"`
	Transcript  []JigasiTranscriptAlternative `json:"transcript"`
	Language    string                        `json:"language"`
	MessageID   string                        `json:"message_id"`
	IsInterim   bool                          `json:"is_interim"`
	Stability   float64                       `json:"stability"`
}
