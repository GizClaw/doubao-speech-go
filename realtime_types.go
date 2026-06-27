package doubaospeech

import "time"

// RealtimeEventType represents realtime websocket event ID.
type RealtimeEventType int32

const (
	// Connection events.
	EventConnectionStarted RealtimeEventType = 50
	EventConnectionFailed  RealtimeEventType = 51
	EventConnectionEnded   RealtimeEventType = 52

	// Session events.
	EventSessionStarted  RealtimeEventType = 150
	EventSessionFinished RealtimeEventType = 152
	EventSessionFailed   RealtimeEventType = 153
	EventUsageResponse   RealtimeEventType = 154
	EventConfigUpdated   RealtimeEventType = 251

	// ASR events.
	EventASRInfo     RealtimeEventType = 450
	EventASRResponse RealtimeEventType = 451
	EventASREnded    RealtimeEventType = 459

	// TTS events.
	EventTTSStarted    RealtimeEventType = 350
	EventTTSSegmentEnd RealtimeEventType = 351
	EventTTSAudioData  RealtimeEventType = 352
	EventTTSFinished   RealtimeEventType = 359

	// Chat events.
	EventChatResponse           RealtimeEventType = 550
	EventChatTextQueryConfirmed RealtimeEventType = 553
	EventChatEnded              RealtimeEventType = 559

	// Conversation events.
	EventConversationCreated   RealtimeEventType = 567
	EventConversationUpdated   RealtimeEventType = 568
	EventConversationRetrieved RealtimeEventType = 569
	EventConversationTruncated RealtimeEventType = 570
	EventConversationDeleted   RealtimeEventType = 571
	EventDialogCommonError     RealtimeEventType = 599
)

// RealtimeInputMode controls how the session receives user input.
type RealtimeInputMode string

const (
	// RealtimeInputModeDefault leaves input_mod unset and uses realtime server-VAD audio.
	RealtimeInputModeDefault RealtimeInputMode = ""
	// RealtimeInputModeKeepAlive keeps muted microphone sessions alive.
	RealtimeInputModeKeepAlive RealtimeInputMode = "keep_alive"
	// RealtimeInputModePushToTalk uses client-controlled end-of-speech.
	RealtimeInputModePushToTalk RealtimeInputMode = "push_to_talk"
	// RealtimeInputModeText sends user turns as text.
	RealtimeInputModeText RealtimeInputMode = "text"
	// RealtimeInputModeAudioFile streams a recording file as timed audio chunks.
	RealtimeInputModeAudioFile RealtimeInputMode = "audio_file"
)

// RealtimeModelVersion selects the realtime model family.
type RealtimeModelVersion string

const (
	RealtimeModelO20  RealtimeModelVersion = "1.2.1.1"
	RealtimeModelSC20 RealtimeModelVersion = "2.2.0.0"
)

// RealtimeConfig represents one realtime session config.
type RealtimeConfig struct {
	ASR    RealtimeASRConfig    `json:"asr" yaml:"asr"`
	TTS    RealtimeTTSConfig    `json:"tts" yaml:"tts"`
	Dialog RealtimeDialogConfig `json:"dialog" yaml:"dialog"`

	Prompt  RealtimePromptConfig          `json:"prompt" yaml:"prompt,omitempty"`
	Props   RealtimeGenerationProps       `json:"props" yaml:"props,omitempty"`
	History []RealtimeConversationMessage `json:"history,omitempty" yaml:"history,omitempty"`

	InputMode RealtimeInputMode    `json:"input_mode,omitempty" yaml:"input_mode,omitempty"`
	Model     RealtimeModelVersion `json:"model,omitempty" yaml:"model,omitempty"`

	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`

	// Local runtime controls (not sent to server).
	EventBuffer         int           `json:"-" yaml:"-"`
	BackpressureTimeout time.Duration `json:"-" yaml:"-"`
}

// RealtimeASRConfig configures ASR behavior.
type RealtimeASRConfig struct {
	Language  Language              `json:"language,omitempty" yaml:"language,omitempty"`
	AudioInfo *RealtimeASRAudioInfo `json:"audio_info,omitempty" yaml:"audio_info,omitempty"`
	Extra     *RealtimeASRExtra     `json:"extra,omitempty" yaml:"extra,omitempty"`
}

// RealtimeASRAudioInfo configures uploaded audio metadata for realtime ASR.
type RealtimeASRAudioInfo struct {
	Format     AudioFormat `json:"format,omitempty" yaml:"format,omitempty"`
	SampleRate SampleRate  `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
	Channel    int         `json:"channel,omitempty" yaml:"channel,omitempty"`
}

// RealtimeASRExtra configures ASR-specific StartSession fields.
type RealtimeASRExtra struct {
	EndSmoothWindowMS     int                 `json:"end_smooth_window_ms,omitempty" yaml:"end_smooth_window_ms,omitempty"`
	EnableCustomVAD       *bool               `json:"enable_custom_vad,omitempty" yaml:"enable_custom_vad,omitempty"`
	EnableASRTwopass      *bool               `json:"enable_asr_twopass,omitempty" yaml:"enable_asr_twopass,omitempty"`
	BoostingTableID       string              `json:"boosting_table_id,omitempty" yaml:"boosting_table_id,omitempty"`
	BoostingTableName     string              `json:"boosting_table_name,omitempty" yaml:"boosting_table_name,omitempty"`
	RegexCorrectTableID   string              `json:"regex_correct_table_id,omitempty" yaml:"regex_correct_table_id,omitempty"`
	RegexCorrectTableName string              `json:"regex_correct_table_name,omitempty" yaml:"regex_correct_table_name,omitempty"`
	Context               *RealtimeASRContext `json:"context,omitempty" yaml:"context,omitempty"`
}

// RealtimeASRContext configures inline ASR hotwords and replacement rules.
type RealtimeASRContext struct {
	Hotwords     []RealtimeHotword `json:"hotwords,omitempty" yaml:"hotwords,omitempty"`
	CorrectWords map[string]string `json:"correct_words,omitempty" yaml:"correct_words,omitempty"`
}

// RealtimeHotword is one inline ASR hotword.
type RealtimeHotword struct {
	Word string `json:"word" yaml:"word"`
}

// RealtimeTTSConfig configures TTS behavior.
type RealtimeTTSConfig struct {
	Speaker     string              `json:"speaker" yaml:"speaker"`
	AudioConfig RealtimeAudioConfig `json:"audio_config" yaml:"audio_config"`
	Extra       *RealtimeTTSExtra   `json:"extra,omitempty" yaml:"extra,omitempty"`
}

// RealtimeAudioConfig describes audio IO parameters.
type RealtimeAudioConfig struct {
	Channel      int         `json:"channel" yaml:"channel"`
	Format       AudioFormat `json:"format" yaml:"format"`
	SampleRate   SampleRate  `json:"sample_rate" yaml:"sample_rate"`
	Bits         int         `json:"bits,omitempty" yaml:"bits,omitempty"`
	SpeechRate   int         `json:"speech_rate,omitempty" yaml:"speech_rate,omitempty"`
	LoudnessRate int         `json:"loudness_rate,omitempty" yaml:"loudness_rate,omitempty"`
}

// RealtimeDialogConfig configures dialogue behavior.
type RealtimeDialogConfig struct {
	DialogID          string                      `json:"dialog_id,omitempty" yaml:"dialog_id,omitempty"`
	BotName           string                      `json:"bot_name,omitempty" yaml:"bot_name,omitempty"`
	SystemRole        string                      `json:"system_role,omitempty" yaml:"system_role,omitempty"`
	SpeakingStyle     string                      `json:"speaking_style,omitempty" yaml:"speaking_style,omitempty"`
	CharacterManifest string                      `json:"character_manifest,omitempty" yaml:"character_manifest,omitempty"`
	Location          *RealtimeLocation           `json:"location,omitempty" yaml:"location,omitempty"`
	DialogContext     []RealtimeDialogContextItem `json:"dialog_context,omitempty" yaml:"dialog_context,omitempty"`
	Extra             *RealtimeDialogExtra        `json:"extra,omitempty" yaml:"extra,omitempty"`
}

// RealtimeLocation configures user location for realtime web-search precision.
type RealtimeLocation struct {
	Longitude   float64 `json:"longitude,omitempty" yaml:"longitude,omitempty"`
	Latitude    float64 `json:"latitude,omitempty" yaml:"latitude,omitempty"`
	City        string  `json:"city,omitempty" yaml:"city,omitempty"`
	Country     string  `json:"country,omitempty" yaml:"country,omitempty"`
	Province    string  `json:"province,omitempty" yaml:"province,omitempty"`
	District    string  `json:"district,omitempty" yaml:"district,omitempty"`
	Town        string  `json:"town,omitempty" yaml:"town,omitempty"`
	CountryCode string  `json:"country_code,omitempty" yaml:"country_code,omitempty"`
	Address     string  `json:"address,omitempty" yaml:"address,omitempty"`
}

// RealtimeDialogContextItem is one initial dialogue context item.
type RealtimeDialogContextItem struct {
	Role      string `json:"role" yaml:"role"`
	Text      string `json:"text" yaml:"text"`
	Timestamp int64  `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

// RealtimeDialogExtra configures dialogue-specific StartSession fields.
type RealtimeDialogExtra struct {
	StrictAudit                  *bool  `json:"strict_audit,omitempty" yaml:"strict_audit,omitempty"`
	AuditResponse                string `json:"audit_response,omitempty" yaml:"audit_response,omitempty"`
	EnableVolcWebsearch          *bool  `json:"enable_volc_websearch,omitempty" yaml:"enable_volc_websearch,omitempty"`
	VolcWebsearchType            string `json:"volc_websearch_type,omitempty" yaml:"volc_websearch_type,omitempty"`
	VolcWebsearchAPIKey          string `json:"volc_websearch_api_key,omitempty" yaml:"volc_websearch_api_key,omitempty"`
	VolcWebsearchBotID           string `json:"volc_websearch_bot_id,omitempty" yaml:"volc_websearch_bot_id,omitempty"`
	VolcWebsearchResultCount     int    `json:"volc_websearch_result_count,omitempty" yaml:"volc_websearch_result_count,omitempty"`
	VolcWebsearchNoResultMessage string `json:"volc_websearch_no_result_message,omitempty" yaml:"volc_websearch_no_result_message,omitempty"`
	EnableMusic                  *bool  `json:"enable_music,omitempty" yaml:"enable_music,omitempty"`
	EnableLoudnessNorm           *bool  `json:"enable_loudness_norm,omitempty" yaml:"enable_loudness_norm,omitempty"`
	EnableConversationTruncate   *bool  `json:"enable_conversation_truncate,omitempty" yaml:"enable_conversation_truncate,omitempty"`
	EnableUserQueryExit          *bool  `json:"enable_user_query_exit,omitempty" yaml:"enable_user_query_exit,omitempty"`
}

// RealtimeTTSExtra configures TTS-specific StartSession fields.
type RealtimeTTSExtra struct {
	ExplicitDialect string                `json:"explicit_dialect,omitempty" yaml:"explicit_dialect,omitempty"`
	AIGCMetadata    *RealtimeAIGCMetadata `json:"aigc_metadata,omitempty" yaml:"aigc_metadata,omitempty"`
	TTS20Model      string                `json:"tts_2.0_model,omitempty" yaml:"tts_2.0_model,omitempty"`
}

// RealtimeAIGCMetadata configures hidden AIGC metadata watermarking.
type RealtimeAIGCMetadata struct {
	Enable            *bool  `json:"enable,omitempty" yaml:"enable,omitempty"`
	ContentProducer   string `json:"content_producer,omitempty" yaml:"content_producer,omitempty"`
	ProduceID         string `json:"produce_id,omitempty" yaml:"produce_id,omitempty"`
	ContentPropagator string `json:"content_propagator,omitempty" yaml:"content_propagator,omitempty"`
	PropagateID       string `json:"propagate_id,omitempty" yaml:"propagate_id,omitempty"`
}

// RealtimePromptConfig controls prompt and prompt variables.
type RealtimePromptConfig struct {
	System    string            `json:"system,omitempty" yaml:"system,omitempty"`
	Variables map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// RealtimeGenerationProps controls generation params.
type RealtimeGenerationProps struct {
	Temperature      float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP             float64 `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	MaxTokens        int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	PresencePenalty  float64 `json:"presence_penalty,omitempty" yaml:"presence_penalty,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
}

// RealtimeConversationMessage is one dialog history entry.
type RealtimeConversationMessage struct {
	Role    string `json:"role" yaml:"role"`
	Content string `json:"content" yaml:"content"`
}

// RealtimeConversationItem is one server-side conversation context item.
type RealtimeConversationItem struct {
	ItemID    string `json:"item_id,omitempty" yaml:"item_id,omitempty"`
	Role      string `json:"role,omitempty" yaml:"role,omitempty"`
	Text      string `json:"text,omitempty" yaml:"text,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

// RealtimeUpdateConfig is the full-replacement payload for UpdateConfig.
type RealtimeUpdateConfig struct {
	TTS    *RealtimeTTSConfig    `json:"tts,omitempty" yaml:"tts,omitempty"`
	Dialog *RealtimeDialogConfig `json:"dialog,omitempty" yaml:"dialog,omitempty"`
}

// RealtimeEvent is one parsed server event.
type RealtimeEvent struct {
	Type      RealtimeEventType `json:"type"`
	SessionID string            `json:"session_id,omitempty"`
	DialogID  string            `json:"dialog_id,omitempty"`
	ConnectID string            `json:"connect_id,omitempty"`
	Sequence  int32             `json:"sequence,omitempty"`

	Text    string `json:"text,omitempty"`
	Audio   []byte `json:"audio,omitempty"`
	Payload []byte `json:"payload,omitempty"`

	QuestionID string                     `json:"question_id,omitempty"`
	ReplyID    string                     `json:"reply_id,omitempty"`
	TTSType    string                     `json:"tts_type,omitempty"`
	StatusCode string                     `json:"status_code,omitempty"`
	Results    []RealtimeASRResult        `json:"results,omitempty"`
	Usage      *RealtimeUsage             `json:"usage,omitempty"`
	Items      []RealtimeConversationItem `json:"items,omitempty"`
	Message    string                     `json:"message,omitempty"`

	Error   *Error `json:"error,omitempty"`
	IsFinal bool   `json:"is_final,omitempty"`

	ReqID   string `json:"reqid,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
	LogID   string `json:"log_id,omitempty"`
}

// RealtimeASRResult is one ASR hypothesis returned by the realtime service.
type RealtimeASRResult struct {
	Text      string `json:"text"`
	IsInterim bool   `json:"is_interim"`
}

// RealtimeUsage contains token usage reported by a realtime response.
type RealtimeUsage struct {
	InputTextTokens   int `json:"input_text_tokens,omitempty"`
	InputAudioTokens  int `json:"input_audio_tokens,omitempty"`
	CachedTextTokens  int `json:"cached_text_tokens,omitempty"`
	CachedAudioTokens int `json:"cached_audio_tokens,omitempty"`
	OutputTextTokens  int `json:"output_text_tokens,omitempty"`
	OutputAudioTokens int `json:"output_audio_tokens,omitempty"`
}

// DefaultRealtimeConfig returns a baseline realtime config.
func DefaultRealtimeConfig() RealtimeConfig {
	return RealtimeConfig{
		ASR: RealtimeASRConfig{
			Language: LanguageZhCN,
		},
		TTS: RealtimeTTSConfig{
			Speaker: "zh_female_cancan",
			AudioConfig: RealtimeAudioConfig{
				Channel:    1,
				Format:     FormatPCM,
				SampleRate: SampleRate16000,
				Bits:       16,
			},
		},
		Dialog: RealtimeDialogConfig{},
	}
}
