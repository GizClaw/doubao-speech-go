package doubaospeech

import "time"

type ASTTranslateMode string

const (
	ASTTranslateModeS2T ASTTranslateMode = "s2t"
	ASTTranslateModeS2S ASTTranslateMode = "s2s"
)

type ASTTranslateEventType int32

const (
	ASTEventSessionStarted  ASTTranslateEventType = 150
	ASTEventSessionCanceled ASTTranslateEventType = 151
	ASTEventSessionFinished ASTTranslateEventType = 152
	ASTEventSessionFailed   ASTTranslateEventType = 153
	ASTEventUsageResponse   ASTTranslateEventType = 154
	ASTEventAudioMuted      ASTTranslateEventType = 250

	ASTEventTTSSentenceStart            ASTTranslateEventType = 350
	ASTEventTTSSentenceEnd              ASTTranslateEventType = 351
	ASTEventTTSResponse                 ASTTranslateEventType = 352
	ASTEventSourceSubtitleStart         ASTTranslateEventType = 650
	ASTEventSourceSubtitleResponse      ASTTranslateEventType = 651
	ASTEventSourceSubtitleEnd           ASTTranslateEventType = 652
	ASTEventTranslationSubtitleStart    ASTTranslateEventType = 653
	ASTEventTranslationSubtitleResponse ASTTranslateEventType = 654
	ASTEventTranslationSubtitleEnd      ASTTranslateEventType = 655
)

type ASTTranslateConfig struct {
	SessionID  string           `json:"session_id,omitempty" yaml:"session_id,omitempty"`
	ResourceID string           `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`
	Mode       ASTTranslateMode `json:"mode" yaml:"mode"`

	SourceLanguage string `json:"source_language" yaml:"source_language"`
	TargetLanguage string `json:"target_language" yaml:"target_language"`

	SourceAudio ASTAudioConfig       `json:"source_audio" yaml:"source_audio"`
	TargetAudio ASTTargetAudioConfig `json:"target_audio" yaml:"target_audio,omitempty"`

	SpeakerID                  string `json:"speaker_id,omitempty" yaml:"speaker_id,omitempty"`
	IsCustomSpeaker            bool   `json:"is_custom_speaker,omitempty" yaml:"is_custom_speaker,omitempty"`
	TTSResourceID              string `json:"tts_resource_id,omitempty" yaml:"tts_resource_id,omitempty"`
	SpeechRate                 int    `json:"speech_rate,omitempty" yaml:"speech_rate,omitempty"`
	EnableSourceLanguageDetect bool   `json:"enable_source_language_detect,omitempty" yaml:"enable_source_language_detect,omitempty"`
	Denoise                    *bool  `json:"denoise,omitempty" yaml:"denoise,omitempty"`

	Corpus *ASTTranslateCorpus `json:"corpus,omitempty" yaml:"corpus,omitempty"`
	User   ASTUser             `json:"user" yaml:"user,omitempty"`

	EventBuffer         int           `json:"-" yaml:"-"`
	BackpressureTimeout time.Duration `json:"-" yaml:"-"`
}

type ASTTranslateUpdate struct {
	Corpus *ASTTranslateCorpus `json:"corpus,omitempty" yaml:"corpus,omitempty"`
}

type ASTUser struct {
	UID        string `json:"uid,omitempty" yaml:"uid,omitempty"`
	DID        string `json:"did,omitempty" yaml:"did,omitempty"`
	Platform   string `json:"platform,omitempty" yaml:"platform,omitempty"`
	SDKVersion string `json:"sdk_version,omitempty" yaml:"sdk_version,omitempty"`
	AppVersion string `json:"app_version,omitempty" yaml:"app_version,omitempty"`
}

type ASTAudioConfig struct {
	Format  AudioFormat `json:"format" yaml:"format"`
	Codec   string      `json:"codec,omitempty" yaml:"codec,omitempty"`
	Rate    SampleRate  `json:"rate" yaml:"rate"`
	Bits    int         `json:"bits" yaml:"bits"`
	Channel int         `json:"channel" yaml:"channel"`
}

type ASTTargetAudioConfig struct {
	Format  AudioFormat `json:"format,omitempty" yaml:"format,omitempty"`
	Rate    SampleRate  `json:"rate,omitempty" yaml:"rate,omitempty"`
	Bits    int         `json:"bits,omitempty" yaml:"bits,omitempty"`
	Channel int         `json:"channel,omitempty" yaml:"channel,omitempty"`
}

type ASTTranslateCorpus struct {
	HotWords              []string          `json:"hot_words_list,omitempty" yaml:"hot_words_list,omitempty"`
	BoostingTableID       string            `json:"boosting_table_id,omitempty" yaml:"boosting_table_id,omitempty"`
	BoostingTableName     string            `json:"boosting_table_name,omitempty" yaml:"boosting_table_name,omitempty"`
	CorrectWords          map[string]string `json:"correct_words,omitempty" yaml:"correct_words,omitempty"`
	RegexCorrectTableID   string            `json:"regex_correct_table_id,omitempty" yaml:"regex_correct_table_id,omitempty"`
	RegexCorrectTableName string            `json:"regex_correct_table_name,omitempty" yaml:"regex_correct_table_name,omitempty"`
	Glossary              map[string]string `json:"glossary_list,omitempty" yaml:"glossary_list,omitempty"`
	GlossaryTableID       string            `json:"glossary_table_id,omitempty" yaml:"glossary_table_id,omitempty"`
	GlossaryTableName     string            `json:"glossary_table_name,omitempty" yaml:"glossary_table_name,omitempty"`
}

type ASTTranslateUsage struct {
	Items      []ASTTranslateBillingItem `json:"items,omitempty"`
	DurationMS int64                     `json:"duration_ms,omitempty"`
	WordCount  int64                     `json:"word_count,omitempty"`
}

type ASTTranslateBillingItem struct {
	Unit     string  `json:"unit"`
	Quantity float32 `json:"quantity"`
}

type ASTTranslateEvent struct {
	Type      ASTTranslateEventType `json:"type"`
	SessionID string                `json:"session_id,omitempty"`

	Text             string `json:"text,omitempty"`
	Audio            []byte `json:"-"`
	StartTimeMS      int    `json:"start_time_ms,omitempty"`
	EndTimeMS        int    `json:"end_time_ms,omitempty"`
	SpeakerChanged   bool   `json:"speaker_changed,omitempty"`
	DetectedLanguage string `json:"detected_language,omitempty"`
	MutedDurationMS  int    `json:"muted_duration_ms,omitempty"`

	Usage   *ASTTranslateUsage `json:"usage,omitempty"`
	Error   *Error             `json:"error,omitempty"`
	IsFinal bool               `json:"is_final,omitempty"`

	Payload []byte `json:"payload,omitempty"`
	ReqID   string `json:"reqid,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
	LogID   string `json:"log_id,omitempty"`
}

func DefaultASTTranslateConfig() ASTTranslateConfig {
	denoise := false
	return ASTTranslateConfig{
		Mode:           ASTTranslateModeS2T,
		SourceLanguage: "zh",
		TargetLanguage: "en",
		SourceAudio: ASTAudioConfig{
			Format:  FormatWAV,
			Codec:   "raw",
			Rate:    SampleRate16000,
			Bits:    16,
			Channel: 1,
		},
		Denoise: &denoise,
	}
}
