package doubaospeech

// AudioFormat represents audio encoding format.
type AudioFormat string

const (
	FormatPCM        AudioFormat = "pcm"
	FormatPCMS16LE   AudioFormat = "pcm_s16le"
	FormatWAV        AudioFormat = "wav"
	FormatMP3        AudioFormat = "mp3"
	FormatOGG        AudioFormat = "ogg_opus"
	FormatAAC        AudioFormat = "aac"
	FormatM4A        AudioFormat = "m4a"
	FormatSpeechOpus AudioFormat = "speech_opus"
)

// SampleRate represents audio sample rate.
type SampleRate int

const (
	SampleRate8000  SampleRate = 8000
	SampleRate16000 SampleRate = 16000
	SampleRate22050 SampleRate = 22050
	SampleRate24000 SampleRate = 24000
	SampleRate32000 SampleRate = 32000
	SampleRate44100 SampleRate = 44100
	SampleRate48000 SampleRate = 48000
)

// Language represents recognition language.
type Language string

const (
	LanguageZhCN Language = "zh-CN"
	LanguageEnUS Language = "en-US"
	LanguageJaJP Language = "ja-JP"
	LanguageKoKR Language = "ko-KR"
)

// TaskStatus is async task status.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusSuccess    TaskStatus = "success"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// ASRV2Config is SAUC V2 streaming session config.
type ASRV2Config struct {
	Format     AudioFormat      `json:"format" yaml:"format"`
	SampleRate SampleRate       `json:"sample_rate" yaml:"sample_rate"`
	Channel    int              `json:"channel,omitempty" yaml:"channel,omitempty"`
	Channels   int              `json:"channels,omitempty" yaml:"channels,omitempty"` // Backward-compatible alias field.
	Bits       int              `json:"bits,omitempty" yaml:"bits,omitempty"`
	Language   Language         `json:"language,omitempty" yaml:"language,omitempty"`
	Codec      ASRV2AudioCodec  `json:"codec,omitempty" yaml:"codec,omitempty"`
	User       *ASRV2UserConfig `json:"user,omitempty" yaml:"user,omitempty"`

	EnableITN         bool     `json:"enable_itn,omitempty" yaml:"enable_itn,omitempty"`
	EnablePunc        bool     `json:"enable_punc,omitempty" yaml:"enable_punc,omitempty"`
	EnableDiarization bool     `json:"enable_diarization,omitempty" yaml:"enable_diarization,omitempty"`
	SpeakerNum        int      `json:"speaker_num,omitempty" yaml:"speaker_num,omitempty"`
	Hotwords          []string `json:"hotwords,omitempty" yaml:"hotwords,omitempty"`
	ResultType        string   `json:"result_type,omitempty" yaml:"result_type,omitempty"` // single/full

	// Request contains the complete typed BigASR request configuration. Its
	// fields take precedence over the legacy request fields above.
	Request *ASRV2RequestConfig `json:"request,omitempty" yaml:"request,omitempty"`

	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`
}

// ASRV2AudioCodec is the encoded audio codec declared to BigASR.
type ASRV2AudioCodec string

const (
	ASRV2AudioCodecRaw  ASRV2AudioCodec = "raw"
	ASRV2AudioCodecOpus ASRV2AudioCodec = "opus"
)

// ASRV2UserConfig contains optional client metadata used for log filtering.
// UID falls back to the client user ID when it is empty.
type ASRV2UserConfig struct {
	UID        string `json:"uid,omitempty" yaml:"uid,omitempty"`
	DID        string `json:"did,omitempty" yaml:"did,omitempty"`
	Platform   string `json:"platform,omitempty" yaml:"platform,omitempty"`
	SDKVersion string `json:"sdk_version,omitempty" yaml:"sdk_version,omitempty"`
	AppVersion string `json:"app_version,omitempty" yaml:"app_version,omitempty"`
}

// ASRV2RequestConfig contains the typed request fields supported by BigASR.
// Pointer scalars distinguish an omitted field from an explicit false or zero.
type ASRV2RequestConfig struct {
	ModelName              string `json:"model_name,omitempty" yaml:"model_name,omitempty"`
	EnableNonstream        *bool  `json:"enable_nonstream,omitempty" yaml:"enable_nonstream,omitempty"`
	EnableITN              *bool  `json:"enable_itn,omitempty" yaml:"enable_itn,omitempty"`
	EnableSpeakerInfo      *bool  `json:"enable_speaker_info,omitempty" yaml:"enable_speaker_info,omitempty"`
	SSDVersion             string `json:"ssd_version,omitempty" yaml:"ssd_version,omitempty"`
	EnablePunc             *bool  `json:"enable_punc,omitempty" yaml:"enable_punc,omitempty"`
	EnableDDC              *bool  `json:"enable_ddc,omitempty" yaml:"enable_ddc,omitempty"`
	OutputZHVariant        string `json:"output_zh_variant,omitempty" yaml:"output_zh_variant,omitempty"`
	EnableAutoLanguage     *bool  `json:"enable_auto_lang,omitempty" yaml:"enable_auto_lang,omitempty"`
	ShowUtterances         *bool  `json:"show_utterances,omitempty" yaml:"show_utterances,omitempty"`
	ShowSpeechRate         *bool  `json:"show_speech_rate,omitempty" yaml:"show_speech_rate,omitempty"`
	ShowVolume             *bool  `json:"show_volume,omitempty" yaml:"show_volume,omitempty"`
	EnableLanguageID       *bool  `json:"enable_lid,omitempty" yaml:"enable_lid,omitempty"`
	EnableEmotionDetection *bool  `json:"enable_emotion_detection,omitempty" yaml:"enable_emotion_detection,omitempty"`
	EnableGenderDetection  *bool  `json:"enable_gender_detection,omitempty" yaml:"enable_gender_detection,omitempty"`
	ResultType             string `json:"result_type,omitempty" yaml:"result_type,omitempty"`
	EnableAccelerateText   *bool  `json:"enable_accelerate_text,omitempty" yaml:"enable_accelerate_text,omitempty"`
	AccelerateScore        *int   `json:"accelerate_score,omitempty" yaml:"accelerate_score,omitempty"`
	// VADSegmentDuration is the semantic segmentation maximum silence duration
	// in milliseconds. The service default is 3000. It is ignored when
	// EndWindowSize is set.
	VADSegmentDuration *int `json:"vad_segment_duration,omitempty" yaml:"vad_segment_duration,omitempty"`
	// EndWindowSize is the forced endpointing silence duration in milliseconds.
	// The service default is 800 and the minimum supported value is 200.
	EndWindowSize *int `json:"end_window_size,omitempty" yaml:"end_window_size,omitempty"`
	// ForceToSpeechTime is the minimum audio duration in milliseconds before
	// forced endpointing can occur. It requires EndWindowSize to be set.
	ForceToSpeechTime    *int               `json:"force_to_speech_time,omitempty" yaml:"force_to_speech_time,omitempty"`
	SensitiveWordsFilter *string            `json:"sensitive_words_filter,omitempty" yaml:"sensitive_words_filter,omitempty"`
	EnablePOIFC          *bool              `json:"enable_poi_fc,omitempty" yaml:"enable_poi_fc,omitempty"`
	EnableMusicFC        *bool              `json:"enable_music_fc,omitempty" yaml:"enable_music_fc,omitempty"`
	Corpus               *ASRV2CorpusConfig `json:"corpus,omitempty" yaml:"corpus,omitempty"`
}

// ASRV2CorpusConfig configures BigASR hotword, correction, and context data.
type ASRV2CorpusConfig struct {
	BoostingTableName string              `json:"boosting_table_name,omitempty" yaml:"boosting_table_name,omitempty"`
	BoostingTableID   string              `json:"boosting_table_id,omitempty" yaml:"boosting_table_id,omitempty"`
	CorrectTableName  string              `json:"correct_table_name,omitempty" yaml:"correct_table_name,omitempty"`
	CorrectTableID    string              `json:"correct_table_id,omitempty" yaml:"correct_table_id,omitempty"`
	Context           *ASRV2CorpusContext `json:"context,omitempty" yaml:"context,omitempty"`
}

// ASRV2CorpusContext is encoded as the JSON string required by request.corpus.context.
type ASRV2CorpusContext struct {
	Hotwords     []ASRV2Hotword        `json:"hotwords,omitempty" yaml:"hotwords,omitempty"`
	CorrectWords []ASRV2WordCorrection `json:"correct_words,omitempty" yaml:"correct_words,omitempty"`
	ContextType  string                `json:"context_type,omitempty" yaml:"context_type,omitempty"`
	ContextData  []ASRV2ContextEntry   `json:"context_data,omitempty" yaml:"context_data,omitempty"`
}

// ASRV2Hotword is one directly supplied BigASR hotword.
type ASRV2Hotword struct {
	Word string `json:"word" yaml:"word"`
}

// ASRV2WordCorrection replaces Source with Target during recognition.
type ASRV2WordCorrection struct {
	Source string `json:"source" yaml:"source"`
	Target string `json:"target" yaml:"target"`
}

// ASRV2ContextEntry is one textual or visual dialogue-context item.
type ASRV2ContextEntry struct {
	Text     string `json:"text,omitempty" yaml:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty" yaml:"image_url,omitempty"`
}

// ASRV2Result is one parsed server response.
type ASRV2Result struct {
	Text       string           `json:"text"`
	Utterances []ASRV2Utterance `json:"utterances,omitempty"`
	IsFinal    bool             `json:"is_final"`
	Duration   int              `json:"duration,omitempty"`
	ReqID      string           `json:"reqid,omitempty"`
	TraceID    string           `json:"trace_id,omitempty"`
	LogID      string           `json:"log_id,omitempty"`
	ConnectID  string           `json:"connect_id,omitempty"`
}

// ASRV2Utterance contains utterance-level info.
type ASRV2Utterance struct {
	Text       string      `json:"text"`
	StartTime  int         `json:"start_time"`
	EndTime    int         `json:"end_time"`
	Definite   bool        `json:"definite"`
	SpeakerID  string      `json:"speaker_id,omitempty"`
	Words      []ASRV2Word `json:"words,omitempty"`
	Confidence float64     `json:"confidence,omitempty"`
}

// ASRV2Word contains word-level timing info.
type ASRV2Word struct {
	Text      string  `json:"text"`
	StartTime int     `json:"start_time"`
	EndTime   int     `json:"end_time"`
	Conf      float64 `json:"conf,omitempty"`
}

// Backward-compatible aliases mapped to V2 types.
type StreamASRConfig = ASRV2Config
type ASRChunk = ASRV2Result
type Utterance = ASRV2Utterance
type Word = ASRV2Word

// TTSV2WSConfig is bidirectional TTS V2 WebSocket session config.
type TTSV2WSConfig struct {
	Speaker    string      `json:"speaker" yaml:"speaker"`
	Format     AudioFormat `json:"format,omitempty" yaml:"format,omitempty"`
	SampleRate SampleRate  `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`

	// ResourceID defaults to seed-tts-2.0 when empty.
	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`
}

// TTSV2WSChunk is one downstream chunk from TTS V2 WebSocket stream.
type TTSV2WSChunk struct {
	Audio     []byte `json:"-"`
	IsFinal   bool   `json:"is_final"`
	Event     int32  `json:"event"`
	ReqID     string `json:"reqid,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	LogID     string `json:"log_id,omitempty"`
	ConnectID string `json:"connect_id,omitempty"`
}
