package enums

type AIProvider string

const (
	AIProviderOpenAI  AIProvider = "openai"
	AIProviderSub2API AIProvider = "sub2api"
)

var aiProviderLabelMap = map[AIProvider]string{
	AIProviderOpenAI:  "OpenAI",
	AIProviderSub2API: "Sub2API",
}

func GetAIProviderLabel(provider AIProvider) string {
	return aiProviderLabelMap[provider]
}

type AIModelType string

const (
	AIModelTypeLLM       AIModelType = "llm"
	AIModelTypeEmbedding AIModelType = "embedding"
	AIModelTypeRerank    AIModelType = "rerank"
)

var aiModelTypeLabelMap = map[AIModelType]string{
	AIModelTypeLLM:       "enum.AIModelTypeLLM",
	AIModelTypeEmbedding: "enum.AIModelTypeEmbedding",
	AIModelTypeRerank:    "enum.AIModelTypeRerank",
}

func GetAIModelTypeLabel(modelType AIModelType, locale string) string {
	return enumLabel(locale, aiModelTypeLabelMap[modelType])
}
