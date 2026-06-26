package doubaospeech

const (
	// RealtimeDuplexModelDefault is the model version used by the official duplex sample.
	RealtimeDuplexModelDefault = "1.2.6.0"

	RealtimeDuplexAudioPCM      = "pcm"
	RealtimeDuplexAudioPCMS16LE = "pcm_s16le"
	RealtimeDuplexAudioOggOpus  = "ogg_opus"
	RealtimeDuplexAudioOpus     = "speech_opus"

	defaultRealtimeDuplexInputRate  = 16000
	defaultRealtimeDuplexOutputRate = 24000
)

// RealtimeDuplexConfig configures one realtime duplex dialogue session.
type RealtimeDuplexConfig struct {
	Session   RealtimeDuplexSessionConfig `json:"session"`
	Extension *RealtimeDuplexExtension    `json:"extension,omitempty"`
}

// RealtimeDuplexSessionConfig is the session object sent with session.create/session.update.
type RealtimeDuplexSessionConfig struct {
	ID           string                       `json:"id,omitempty"`
	Model        string                       `json:"model,omitempty"`
	Instructions string                       `json:"instructions,omitempty"`
	Audio        RealtimeDuplexAudioConfig    `json:"audio"`
	Tools        []RealtimeDuplexFunctionTool `json:"tools,omitempty"`
}

// RealtimeDuplexAudioConfig configures input and output audio.
type RealtimeDuplexAudioConfig struct {
	Input  RealtimeDuplexAudioInputConfig  `json:"input"`
	Output RealtimeDuplexAudioOutputConfig `json:"output"`
}

// RealtimeDuplexAudioInputConfig configures client audio sent to the service.
type RealtimeDuplexAudioInputConfig struct {
	Format RealtimeDuplexAudioFormat `json:"format"`
}

// RealtimeDuplexAudioOutputConfig configures service audio output.
type RealtimeDuplexAudioOutputConfig struct {
	Format   RealtimeDuplexAudioFormat `json:"format"`
	Speed    int                       `json:"speed,omitempty"`
	Loudness int                       `json:"loudness,omitempty"`
	Voice    string                    `json:"voice,omitempty"`
}

// RealtimeDuplexAudioFormat identifies an audio codec and sample rate.
type RealtimeDuplexAudioFormat struct {
	Type string `json:"type,omitempty"`
	Rate int    `json:"rate,omitempty"`
}

// RealtimeDuplexExtension carries the typed extension fields supported by this
// SDK. Unknown provider fields are intentionally not exposed as public API.
type RealtimeDuplexExtension struct {
	Dialog *RealtimeDuplexDialogExtension `json:"dialog,omitempty"`
}

// RealtimeDuplexDialogExtension configures dialogue-specific extensions.
type RealtimeDuplexDialogExtension struct {
	Extra *RealtimeDuplexDialogExtra `json:"extra,omitempty"`
}

// RealtimeDuplexDialogExtra configures supported dialogue extra fields.
type RealtimeDuplexDialogExtra struct {
	AuditResponse      string `json:"audit_response,omitempty"`
	EnableLoudnessNorm *bool  `json:"enable_loudness_norm,omitempty"`
	EnableMusic        *bool  `json:"enable_music,omitempty"`
}

// RealtimeDuplexFunctionTool describes a model-callable function.
type RealtimeDuplexFunctionTool struct {
	Type        string                    `json:"type,omitempty"`
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Parameters  *RealtimeDuplexJSONSchema `json:"parameters,omitempty"`
	Strict      *bool                     `json:"strict,omitempty"`
}

// RealtimeDuplexJSONSchema models the JSON Schema subset used by function tools.
type RealtimeDuplexJSONSchema struct {
	Type                 string                               `json:"type,omitempty"`
	Description          string                               `json:"description,omitempty"`
	Properties           map[string]*RealtimeDuplexJSONSchema `json:"properties,omitempty"`
	Required             []string                             `json:"required,omitempty"`
	AdditionalProperties *bool                                `json:"additionalProperties,omitempty"`
	Items                *RealtimeDuplexJSONSchema            `json:"items,omitempty"`
	Enum                 []string                             `json:"enum,omitempty"`
	MinLength            *int                                 `json:"minLength,omitempty"`
	MaxLength            *int                                 `json:"maxLength,omitempty"`
	Minimum              *float64                             `json:"minimum,omitempty"`
	Maximum              *float64                             `json:"maximum,omitempty"`
	AnyOf                []*RealtimeDuplexJSONSchema          `json:"anyOf,omitempty"`
}

// RealtimeDuplexSpeechTextRequest sends or commits synthesized text.
type RealtimeDuplexSpeechTextRequest struct {
	EventID   string `json:"event_id,omitempty"`
	SpeechID  string `json:"speech_id,omitempty"`
	Text      string `json:"text,omitempty"`
	TTSPrompt string `json:"tts_prompt,omitempty"`
}

// RealtimeDuplexConversationRole identifies a conversation item role.
type RealtimeDuplexConversationRole string

const (
	RealtimeDuplexRoleUser      RealtimeDuplexConversationRole = "user"
	RealtimeDuplexRoleAssistant RealtimeDuplexConversationRole = "assistant"
	RealtimeDuplexRoleTool      RealtimeDuplexConversationRole = "tool"
)

// RealtimeDuplexConversationContent is one message content block.
type RealtimeDuplexConversationContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// RealtimeDuplexConversationItem is one context-management item.
type RealtimeDuplexConversationItem struct {
	ID      string                              `json:"id,omitempty"`
	Type    string                              `json:"type,omitempty"`
	Role    RealtimeDuplexConversationRole      `json:"role,omitempty"`
	CallID  string                              `json:"call_id,omitempty"`
	Status  string                              `json:"status,omitempty"`
	Content []RealtimeDuplexConversationContent `json:"content,omitempty"`
}

// RealtimeDuplexFunctionCall is one function-call request from the service.
type RealtimeDuplexFunctionCall struct {
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// RealtimeDuplexFunctionCallOutput sends one function-call result back to the service.
type RealtimeDuplexFunctionCallOutput struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// DefaultRealtimeDuplexConfig returns a minimal file-input friendly duplex config.
func DefaultRealtimeDuplexConfig() RealtimeDuplexConfig {
	return RealtimeDuplexConfig{
		Session: RealtimeDuplexSessionConfig{
			Model: RealtimeDuplexModelDefault,
			Audio: RealtimeDuplexAudioConfig{
				Input: RealtimeDuplexAudioInputConfig{
					Format: RealtimeDuplexAudioFormat{Type: RealtimeDuplexAudioPCM, Rate: defaultRealtimeDuplexInputRate},
				},
				Output: RealtimeDuplexAudioOutputConfig{
					Format: RealtimeDuplexAudioFormat{Type: RealtimeDuplexAudioPCMS16LE, Rate: defaultRealtimeDuplexOutputRate},
				},
			},
		},
	}
}
