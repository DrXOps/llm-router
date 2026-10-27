package router

// PayloadCtx is the context passed to each rule for evaluation.
// It wraps the essential information extracted from the request payload.
type PayloadCtx struct {
	// ModelIDs maps rule target names ("haiku", "deepseek") to actual model IDs.
	ModelIDs map[string]string

	// Messages is the raw message count.
	MessageCount int

	// HasToolCalls indicates if the conversation contains tool_use or tool_result.
	HasToolCalls bool

	// LastUserMessage is the text of the most recent user message.
	LastUserMessage string

	// AllUserMessages is a slice of all user message texts.
	AllUserMessages []string

	// TotalChars is the total character count across all messages.
	TotalChars int

	// ApproxTokens is an approximate token count across all messages.
	ApproxTokens int
}

// NewPayloadCtx creates a PayloadCtx with the given model ID mapping.
func NewPayloadCtx(modelIDs map[string]string) *PayloadCtx {
	if modelIDs == nil {
		modelIDs = make(map[string]string)
	}
	return &PayloadCtx{
		ModelIDs: modelIDs,
	}
}