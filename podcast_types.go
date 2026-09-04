package doubaospeech

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// PodcastAction selects how the service creates the podcast script.
type PodcastAction int

const (
	// PodcastActionFromSource summarizes InputText or InputInfo.InputURL.
	PodcastActionFromSource PodcastAction = 0
	// PodcastActionFromScript synthesizes the caller-provided NLPTexts script.
	PodcastActionFromScript PodcastAction = 3
	// PodcastActionFromPrompt creates a script from PromptText.
	PodcastActionFromPrompt PodcastAction = 4
)

// PodcastSpeakerID identifies a Podcast-compatible TTS, ICL, or dedicated
// Podcast voice.
type PodcastSpeakerID string

const (
	// PodcastSpeakerMizai is the female voice in the Mizai and Dayi pair.
	PodcastSpeakerMizai PodcastSpeakerID = "zh_female_mizaitongxue_v2_saturn_bigtts"
	// PodcastSpeakerDayi is the male voice in the Mizai and Dayi pair.
	PodcastSpeakerDayi PodcastSpeakerID = "zh_male_dayixiansheng_v2_saturn_bigtts"
	// PodcastSpeakerLiufei is one voice in the Liu Fei and Xiaolei pair.
	PodcastSpeakerLiufei PodcastSpeakerID = "zh_male_liufei_v2_saturn_bigtts"
	// PodcastSpeakerXiaolei is one voice in the Liu Fei and Xiaolei pair.
	PodcastSpeakerXiaolei PodcastSpeakerID = "zh_male_xiaolei_v2_saturn_bigtts"
)

// PodcastSpeakerModel identifies the TTS or ICL model used by one speaker.
type PodcastSpeakerModel string

const (
	// PodcastSpeakerModelSeedTTS20Standard selects the standard Seed TTS 2.0
	// model documented for Podcast speaker additions.
	PodcastSpeakerModelSeedTTS20Standard PodcastSpeakerModel = "seed-tts-2.0-standard"
)

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
	// SessionID identifies the WebSocket session. The SDK generates one when empty.
	SessionID string `json:"-"`
	// InputID uniquely identifies the source material within the request.
	InputID string `json:"input_id"`

	// PromptText describes a topic when Action is PodcastActionFromPrompt.
	PromptText string `json:"prompt_text,omitempty"`
	// InputText contains source text when Action is PodcastActionFromSource.
	InputText string `json:"input_text,omitempty"`
	// NLPTexts contains an already-authored two-speaker script when Action is
	// PodcastActionFromScript.
	NLPTexts []PodcastScriptLine `json:"nlp_texts,omitempty"`
	// Action selects source summarization, caller-provided script, or prompt mode.
	// The SDK infers source mode from InputText or InputInfo.InputURL and prompt
	// mode from PromptText when Action is nil.
	Action *PodcastAction `json:"action,omitempty"`

	// UseHeadMusic controls whether the service adds opening music.
	UseHeadMusic *bool `json:"use_head_music,omitempty"`
	// UseTailMusic controls whether the service adds closing music.
	UseTailMusic *bool `json:"use_tail_music,omitempty"`
	// AIGCWatermark controls the provider's implicit AIGC audio watermark.
	AIGCWatermark *bool `json:"aigc_watermark,omitempty"`

	// InputInfo configures URL input, returned artifacts, and content auditing.
	InputInfo *PodcastInputInfo `json:"input_info,omitempty"`
	// AudioConfig configures the generated audio stream.
	AudioConfig *PodcastAudioConfig `json:"audio_config,omitempty"`
	// SpeakerInfo selects exactly two speakers and their model-specific options.
	SpeakerInfo *PodcastSpeakerInfo `json:"speaker_info,omitempty"`
	// RetryInfo resumes an interrupted server task from a durable round.
	RetryInfo *PodcastRetryInfo `json:"retry_info,omitempty"`
}

// PodcastScriptLine is one speaker turn supplied in script mode.
type PodcastScriptLine struct {
	// Speaker is the voice ID for this turn.
	Speaker PodcastSpeakerID `json:"speaker"`
	// Text is the content spoken during this turn.
	Text string `json:"text"`
}

// PodcastInputInfo configures response and audit behavior.
type PodcastInputInfo struct {
	// InputURL points to a web page or downloadable document in source mode.
	InputURL string `json:"input_url,omitempty"`
	// ReturnAudioURL asks the service to include a temporary download URL.
	ReturnAudioURL bool `json:"return_audio_url"`
	// OnlyNLPText requests the generated script without synthesizing audio.
	OnlyNLPText bool `json:"only_nlp_text,omitempty"`
	// StrictAudit enables the provider's stricter content-audit policy.
	StrictAudit bool `json:"strict_audit"`
}

// PodcastAudioConfig configures generated audio.
type PodcastAudioConfig struct {
	// Format is the encoded output format.
	Format AudioFormat `json:"format,omitempty"`
	// SampleRate is the output sample rate in hertz.
	SampleRate SampleRate `json:"sample_rate,omitempty"`
	// SpeechRate ranges from -50 (0.5x) to 100 (2.0x); zero is normal speed.
	SpeechRate int `json:"speech_rate,omitempty"`
}

// PodcastSpeakerInfo selects the two voices used by the podcast.
type PodcastSpeakerInfo struct {
	// RandomOrder allows either selected speaker to speak first.
	RandomOrder bool `json:"random_order"`
	// Speakers contains exactly two Podcast-compatible TTS, ICL, or dedicated
	// Podcast voice IDs.
	Speakers []PodcastSpeakerID `json:"speakers,omitempty"`
	// SpeakerAdditions contains optional TTS model settings keyed by speaker.
	// It is encoded as the provider-required JSON object without exposing a map.
	SpeakerAdditions PodcastSpeakerAdditions `json:"speaker_additions,omitempty"`
}

// PodcastSpeakerAdditions is a typed collection of per-speaker TTS settings.
//
// The wire protocol represents this value as an object whose keys are dynamic
// speaker IDs and whose values are JSON-encoded strings. The SDK intentionally
// exposes a slice so callers do not need map, any, or json.RawMessage values.
type PodcastSpeakerAdditions []PodcastSpeakerAddition

// PodcastSpeakerAddition configures the TTS model used for one speaker.
type PodcastSpeakerAddition struct {
	// Speaker identifies one entry in PodcastSpeakerInfo.Speakers.
	Speaker PodcastSpeakerID `json:"-"`
	// Model selects the TTS or ICL model variant required by this voice.
	// For example, a voice-clone 2.0 speaker can require a 2.0 model value.
	Model PodcastSpeakerModel `json:"model,omitempty"`
}

// MarshalJSON converts the typed collection into PodcastTTS's dynamic-key wire
// object. Each object value is itself a JSON string, as required by the API.
func (a PodcastSpeakerAdditions) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	seenSpeakers := make([]PodcastSpeakerID, 0, len(a))
	for index, addition := range a {
		if addition.Speaker == "" {
			return nil, fmt.Errorf("podcast speaker addition %d has an empty speaker", index)
		}
		if addition.Model == "" {
			return nil, fmt.Errorf("podcast speaker addition %d has an empty model", index)
		}
		if slices.Contains(seenSpeakers, addition.Speaker) {
			return nil, fmt.Errorf("podcast speaker addition %q is duplicated", addition.Speaker)
		}
		seenSpeakers = append(seenSpeakers, addition.Speaker)
		if index > 0 {
			buffer.WriteByte(',')
		}
		speakerJSON, err := json.Marshal(addition.Speaker)
		if err != nil {
			return nil, fmt.Errorf("marshal podcast speaker addition key: %w", err)
		}
		configJSON, err := json.Marshal(struct {
			Model PodcastSpeakerModel `json:"model,omitempty"`
		}{Model: addition.Model})
		if err != nil {
			return nil, fmt.Errorf("marshal podcast speaker addition config: %w", err)
		}
		configStringJSON, err := json.Marshal(string(configJSON))
		if err != nil {
			return nil, fmt.Errorf("marshal podcast speaker addition value: %w", err)
		}
		buffer.Write(speakerJSON)
		buffer.WriteByte(':')
		buffer.Write(configStringJSON)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

// PodcastRetryInfo resumes generation after the last durable round.
type PodcastRetryInfo struct {
	// TaskID is the task identifier returned by PodcastSession.TaskID.
	TaskID string `json:"retry_task_id"`
	// LastFinishedRoundID is the last fully persisted round.
	LastFinishedRoundID int `json:"last_finished_round_id"`
}

// PodcastEvent is one normalized podcast stream event.
type PodcastEvent struct {
	// Type identifies the normalized server event.
	Type PodcastEventType `json:"type"`
	// SessionID identifies the Podcast WebSocket session.
	SessionID string `json:"session_id,omitempty"`
	// RoundID identifies the current generated conversation turn.
	RoundID int `json:"round_id,omitempty"`
	// Speaker is the voice ID assigned to the current round.
	Speaker PodcastSpeakerID `json:"speaker,omitempty"`
	// Text is the generated script for the current round.
	Text string `json:"text,omitempty"`
	// IsError reports whether a completed round failed.
	IsError bool `json:"is_error,omitempty"`
	// Audio contains bytes from an audio event and is not JSON encoded.
	Audio []byte `json:"-"`
	// Payload preserves the original event payload for diagnostics.
	Payload []byte `json:"payload,omitempty"`
}
