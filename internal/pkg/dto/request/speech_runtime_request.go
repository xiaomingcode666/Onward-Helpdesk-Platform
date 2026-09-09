package request

type SpeechRuntimeUpdateRequest struct {
	Provider                       string  `json:"provider"`
	XfyunAppID                     string  `json:"xfyunAppId"`
	XfyunAPIKey                    string  `json:"xfyunApiKey"`
	XfyunEndpoint                  *string `json:"xfyunEndpoint"`
	XfyunDomain                    *string `json:"xfyunDomain"`
	XfyunTranslationEnabled        *bool   `json:"xfyunTranslationEnabled"`
	XfyunTranslationAPISecret      string  `json:"xfyunTranslationApiSecret"`
	XfyunTranslationEndpoint       *string `json:"xfyunTranslationEndpoint"`
	XfyunTranslationTargetLanguage *string `json:"xfyunTranslationTargetLanguage"`
	AliyunAPIKey                   string  `json:"aliyunApiKey"`
	AliyunWorkspaceID              *string `json:"aliyunWorkspaceId"`
	AliyunEndpoint                 *string `json:"aliyunEndpoint"`
	AliyunModel                    *string `json:"aliyunModel"`
	MockSegmentDurationMS          int     `json:"mockSegmentDurationMs"`
	JigasiEnabled                  *bool   `json:"jigasiEnabled"`
	JigasiSharedSecret             string  `json:"jigasiSharedSecret"`
	JigasiMaxParticipantStreams    *int    `json:"jigasiMaxParticipantStreams"`
	JigasiMaxConcurrentStreams     *int    `json:"jigasiMaxConcurrentStreams"`
}
