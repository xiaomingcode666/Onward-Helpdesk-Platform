package response

type SpeechRuntimeResponse struct {
	Provider                       string `json:"provider"`
	Configured                     bool   `json:"configured"`
	Enabled                        bool   `json:"enabled"`
	CanUpdate                      bool   `json:"canUpdate"`
	XfyunCredentialsConfigured     bool   `json:"xfyunCredentialsConfigured"`
	XfyunEndpoint                  string `json:"xfyunEndpoint"`
	XfyunDomain                    string `json:"xfyunDomain"`
	XfyunTranslationEnabled        bool   `json:"xfyunTranslationEnabled"`
	XfyunTranslationConfigured     bool   `json:"xfyunTranslationConfigured"`
	XfyunTranslationEndpoint       string `json:"xfyunTranslationEndpoint"`
	XfyunTranslationTargetLanguage string `json:"xfyunTranslationTargetLanguage"`
	AliyunAPIKeyConfigured         bool   `json:"aliyunApiKeyConfigured"`
	AliyunWorkspaceID              string `json:"aliyunWorkspaceId"`
	AliyunEndpoint                 string `json:"aliyunEndpoint"`
	AliyunModel                    string `json:"aliyunModel"`
	MockSegmentDurationMS          int    `json:"mockSegmentDurationMs"`
	JigasiEnabled                  bool   `json:"jigasiEnabled"`
	JigasiSharedSecretConfigured   bool   `json:"jigasiSharedSecretConfigured"`
	JigasiMaxParticipantStreams    int    `json:"jigasiMaxParticipantStreams"`
	JigasiMaxConcurrentStreams     int    `json:"jigasiMaxConcurrentStreams"`
}
