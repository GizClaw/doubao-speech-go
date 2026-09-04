package doubaospeech

import "encoding/json"

// PodcastEventType is a server event emitted during podcast generation.
type PodcastEventType int32

const (
	PodcastEventRoundStarted    PodcastEventType = 360
	PodcastEventAudio           PodcastEventType = 361
	PodcastEventRoundFinished   PodcastEventType = 362
	PodcastEventSessionFinished PodcastEventType = 152
)

// PodcastRequest describes one podcast generation session.
type PodcastRequest struct {
	SessionID string `json:"-"`
	InputID   string `json:"input_id"`

	PromptText string `json:"prompt_text,omitempty"`
	InputText  string `json:"input_text,omitempty"`
	Action     *int   `json:"action,omitempty"`

	UseHeadMusic *bool `json:"use_head_music,omitempty"`
	UseTailMusic *bool `json:"use_tail_music,omitempty"`

	InputInfo   *PodcastInputInfo   `json:"input_info,omitempty"`
	AudioConfig *PodcastAudioConfig `json:"audio_config,omitempty"`
	SpeakerInfo *PodcastSpeakerInfo `json:"speaker_info,omitempty"`
	RetryInfo   *PodcastRetryInfo   `json:"retry_info,omitempty"`
}

// PodcastInputInfo configures response and audit behavior.
type PodcastInputInfo struct {
	ReturnAudioURL bool `json:"return_audio_url"`
	StrictAudit    bool `json:"strict_audit"`
}

// PodcastAudioConfig configures generated audio.
type PodcastAudioConfig struct {
	Format     AudioFormat `json:"format,omitempty"`
	SampleRate SampleRate  `json:"sample_rate,omitempty"`
	SpeechRate int         `json:"speech_rate,omitempty"`
}

// PodcastSpeakerInfo selects podcast voices.
type PodcastSpeakerInfo struct {
	RandomOrder bool              `json:"random_order"`
	Speakers    []json.RawMessage `json:"speakers,omitempty"`
}

// PodcastRetryInfo resumes generation after the last durable round.
type PodcastRetryInfo struct {
	TaskID              string `json:"retry_task_id"`
	LastFinishedRoundID int    `json:"last_finished_round_id"`
}

// PodcastEvent is one normalized podcast stream event.
type PodcastEvent struct {
	Type      PodcastEventType `json:"type"`
	SessionID string           `json:"session_id,omitempty"`
	RoundID   int              `json:"round_id,omitempty"`
	IsError   bool             `json:"is_error,omitempty"`
	Audio     []byte           `json:"-"`
	Payload   []byte           `json:"payload,omitempty"`
}
