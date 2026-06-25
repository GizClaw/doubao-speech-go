package doubaospeech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/GizClaw/doubao-speech-go/internal/util"
)

const (
	audioGenerationCreatePath = "/api/v3/tts/create"

	// ModelSeedAudio10 is the current Audio Generation model identifier.
	ModelSeedAudio10 = "seed-audio-1.0"
)

// AudioGenerationService provides non-streaming audio generation.
type AudioGenerationService struct {
	client *Client
}

func newAudioGenerationService(c *Client) *AudioGenerationService {
	return &AudioGenerationService{client: c}
}

// AudioGenerationCreateRequest is the request payload for Audio Generation.
type AudioGenerationCreateRequest struct {
	Model      string                     `json:"model" yaml:"model"`
	TextPrompt string                     `json:"text_prompt" yaml:"text_prompt"`
	References []AudioGenerationReference `json:"references,omitempty" yaml:"references,omitempty"`

	AudioConfig *AudioGenerationAudioConfig `json:"audio_config,omitempty" yaml:"audio_config,omitempty"`
	Watermark   *AudioGenerationWatermark   `json:"watermark,omitempty" yaml:"watermark,omitempty"`

	// RequestID is sent as X-Api-Request-Id. A value is generated when empty.
	RequestID string `json:"-" yaml:"-"`
}

// AudioGenerationReference is one audio, speaker, or image reference.
type AudioGenerationReference struct {
	Speaker string `json:"speaker,omitempty" yaml:"speaker,omitempty"`

	AudioData string `json:"audio_data,omitempty" yaml:"audio_data,omitempty"`
	AudioURL  string `json:"audio_url,omitempty" yaml:"audio_url,omitempty"`

	ImageData string `json:"image_data,omitempty" yaml:"image_data,omitempty"`
	ImageURL  string `json:"image_url,omitempty" yaml:"image_url,omitempty"`
}

// AudioGenerationAudioConfig controls generated audio output.
type AudioGenerationAudioConfig struct {
	Format       AudioFormat `json:"format,omitempty" yaml:"format,omitempty"`
	SampleRate   SampleRate  `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
	SpeechRate   int         `json:"speech_rate,omitempty" yaml:"speech_rate,omitempty"`
	LoudnessRate int         `json:"loudness_rate,omitempty" yaml:"loudness_rate,omitempty"`
	PitchRate    int         `json:"pitch_rate,omitempty" yaml:"pitch_rate,omitempty"`
}

// AudioGenerationWatermark controls explicit and metadata watermarks.
type AudioGenerationWatermark struct {
	AIGCWatermark *bool                    `json:"aigc_watermark,omitempty" yaml:"aigc_watermark,omitempty"`
	AIGCMetadata  *AudioGenerationAIGCMeta `json:"aigc_metadata,omitempty" yaml:"aigc_metadata,omitempty"`
}

// AudioGenerationAIGCMeta is hidden metadata embedded into generated audio.
type AudioGenerationAIGCMeta struct {
	Enable            *bool  `json:"enable,omitempty" yaml:"enable,omitempty"`
	ContentProducer   string `json:"content_producer,omitempty" yaml:"content_producer,omitempty"`
	ProduceID         string `json:"produce_id,omitempty" yaml:"produce_id,omitempty"`
	ContentPropagator string `json:"content_propagator,omitempty" yaml:"content_propagator,omitempty"`
	PropagateID       string `json:"propagate_id,omitempty" yaml:"propagate_id,omitempty"`
}

// AudioGenerationCreateResponse is the Audio Generation response.
type AudioGenerationCreateResponse struct {
	Code             int     `json:"code"`
	Message          string  `json:"message,omitempty"`
	Audio            []byte  `json:"-"`
	AudioBase64      string  `json:"audio,omitempty"`
	Duration         float64 `json:"duration,omitempty"`
	OriginalDuration float64 `json:"original_duration,omitempty"`
	URL              string  `json:"url,omitempty"`

	ReqID   string `json:"reqid,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
	LogID   string `json:"log_id,omitempty"`

	Extra map[string]json.RawMessage `json:"-"`
}

// Create generates audio with the Audio Generation HTTP API.
func (s *AudioGenerationService) Create(ctx context.Context, req *AudioGenerationCreateRequest) (*AudioGenerationCreateResponse, error) {
	normalized, err := normalizeAudioGenerationCreateRequest(req)
	if err != nil {
		return nil, err
	}

	requestID := strings.TrimSpace(normalized.RequestID)
	if requestID == "" {
		requestID = util.NewReqID("audio")
	}

	bodyBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, wrapError(err, "marshal audio generation request")
	}

	endpoint := s.client.buildEndpoint(audioGenerationCreatePath)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, wrapError(err, "create audio generation request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Request-Id", requestID)
	if s.client.config.appID != "" {
		httpReq.Header.Set("X-Api-App-Id", s.client.config.appID)
	}
	if s.client.config.apiKey != "" {
		httpReq.Header.Set("X-Api-Key", s.client.config.apiKey)
	}

	var resp AudioGenerationCreateResponse
	if err := s.client.doRequest(httpReq, &resp); err != nil {
		return nil, err
	}
	if resp.ReqID == "" {
		resp.ReqID = requestID
	}
	if err := resp.apiError(); err != nil {
		return nil, err
	}

	return &resp, nil
}

func normalizeAudioGenerationCreateRequest(req *AudioGenerationCreateRequest) (*AudioGenerationCreateRequest, error) {
	if req == nil {
		return nil, newAPIError(CodeParamError, "request is nil")
	}

	normalized := *req
	normalized.Model = strings.TrimSpace(normalized.Model)
	if normalized.Model == "" {
		return nil, newAPIError(CodeParamError, "model is required")
	}
	if normalized.Model != ModelSeedAudio10 {
		return nil, newAPIError(CodeParamError, "model must be "+ModelSeedAudio10)
	}
	normalized.TextPrompt = strings.TrimSpace(normalized.TextPrompt)
	if normalized.TextPrompt == "" {
		return nil, newAPIError(CodeParamError, "text_prompt is required")
	}
	if len([]rune(normalized.TextPrompt)) > 2048 {
		return nil, newAPIError(CodeParamError, "text_prompt must be 2048 characters or fewer")
	}

	refs := make([]AudioGenerationReference, 0, len(normalized.References))
	audioRefs := 0
	imageRefs := 0
	for i := range normalized.References {
		ref, kind, err := normalizeAudioGenerationReference(normalized.References[i])
		if err != nil {
			return nil, err
		}
		switch kind {
		case "audio":
			audioRefs++
		case "image":
			imageRefs++
		}
		refs = append(refs, ref)
	}
	if audioRefs > 0 && imageRefs > 0 {
		return nil, newAPIError(CodeParamError, "audio and image references cannot be mixed")
	}
	if audioRefs > 3 {
		return nil, newAPIError(CodeParamError, "references supports at most 3 audio references")
	}
	if imageRefs > 1 {
		return nil, newAPIError(CodeParamError, "references supports at most 1 image reference")
	}
	normalized.References = refs

	if normalized.AudioConfig != nil {
		cfg := *normalized.AudioConfig
		if cfg.Format != "" {
			cfg.Format = AudioFormat(util.NormalizeFormat(string(cfg.Format)))
			if err := util.ValidateFormat(string(cfg.Format)); err != nil {
				return nil, newAPIError(CodeParamError, err.Error())
			}
		}
		if cfg.SampleRate != 0 {
			if err := util.ValidateSampleRate(int(cfg.SampleRate)); err != nil {
				return nil, newAPIError(CodeParamError, err.Error())
			}
		}
		if err := validateIntRange("speech_rate", cfg.SpeechRate, -50, 100); err != nil {
			return nil, err
		}
		if err := validateIntRange("loudness_rate", cfg.LoudnessRate, -50, 100); err != nil {
			return nil, err
		}
		if err := validateIntRange("pitch_rate", cfg.PitchRate, -12, 12); err != nil {
			return nil, err
		}
		normalized.AudioConfig = &cfg
	}

	if normalized.Watermark != nil {
		watermark := *normalized.Watermark
		if watermark.AIGCMetadata != nil {
			meta := *watermark.AIGCMetadata
			meta.ContentProducer = strings.TrimSpace(meta.ContentProducer)
			meta.ProduceID = strings.TrimSpace(meta.ProduceID)
			meta.ContentPropagator = strings.TrimSpace(meta.ContentPropagator)
			meta.PropagateID = strings.TrimSpace(meta.PropagateID)
			watermark.AIGCMetadata = &meta
		}
		normalized.Watermark = &watermark
	}

	normalized.RequestID = strings.TrimSpace(normalized.RequestID)

	return &normalized, nil
}

func normalizeAudioGenerationReference(ref AudioGenerationReference) (AudioGenerationReference, string, error) {
	ref.Speaker = strings.TrimSpace(ref.Speaker)
	ref.AudioData = strings.TrimSpace(ref.AudioData)
	ref.AudioURL = strings.TrimSpace(ref.AudioURL)
	ref.ImageData = strings.TrimSpace(ref.ImageData)
	ref.ImageURL = strings.TrimSpace(ref.ImageURL)

	audioFields := countNonEmpty(ref.Speaker, ref.AudioData, ref.AudioURL)
	imageFields := countNonEmpty(ref.ImageData, ref.ImageURL)

	if audioFields == 0 && imageFields == 0 {
		return ref, "", newAPIError(CodeParamError, "reference must include one of speaker, audio_data, audio_url, image_data, or image_url")
	}
	if audioFields > 0 && imageFields > 0 {
		return ref, "", newAPIError(CodeParamError, "reference cannot include both audio and image fields")
	}
	if audioFields > 1 {
		return ref, "", newAPIError(CodeParamError, "reference must include only one of speaker, audio_data, or audio_url")
	}
	if imageFields > 1 {
		return ref, "", newAPIError(CodeParamError, "reference must include only one of image_data or image_url")
	}
	if audioFields > 0 {
		return ref, "audio", nil
	}
	return ref, "image", nil
}

func validateIntRange(name string, value, minValue, maxValue int) error {
	if value == 0 {
		return nil
	}
	if value < minValue || value > maxValue {
		return newAPIError(CodeParamError, name+" must be in range ["+strconv.Itoa(minValue)+","+strconv.Itoa(maxValue)+"]")
	}
	return nil
}

func countNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func (r *AudioGenerationCreateResponse) UnmarshalJSON(data []byte) error {
	type alias AudioGenerationCreateResponse
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		delete(raw, "code")
		delete(raw, "message")
		delete(raw, "audio")
		delete(raw, "duration")
		delete(raw, "original_duration")
		delete(raw, "url")
		delete(raw, "reqid")
		delete(raw, "request_id")
		delete(raw, "trace_id")
		delete(raw, "log_id")
		delete(raw, "logid")
		if len(raw) > 0 {
			aux.Extra = raw
		}
	}
	meta := parseResponseMetadata(data, responseMetadata{ReqID: aux.ReqID, TraceID: aux.TraceID, LogID: aux.LogID})
	aux.ReqID = meta.ReqID
	aux.TraceID = meta.TraceID
	aux.LogID = meta.LogID
	if payload, ok := parseAPIErrorPayload(data); ok {
		if aux.Code == 0 && (payload.Code != 0 || payload.StatusCode != 0) {
			aux.Code = apiErrorPayloadCode(payload, 0)
		}
		if aux.Message == "" {
			aux.Message = apiErrorPayloadMessage(payload, "")
		}
	}

	aux.AudioBase64 = strings.TrimSpace(aux.AudioBase64)
	if aux.AudioBase64 != "" {
		audio, err := base64.StdEncoding.DecodeString(aux.AudioBase64)
		if err != nil {
			return wrapError(err, "decode audio generation audio")
		}
		aux.Audio = audio
	}

	*r = AudioGenerationCreateResponse(aux)
	return nil
}

func (r *AudioGenerationCreateResponse) setResponseMetadata(meta responseMetadata) {
	if r == nil {
		return
	}
	if r.LogID == "" {
		r.LogID = meta.LogID
	}
}

func (r *AudioGenerationCreateResponse) apiError() error {
	if r == nil {
		return nil
	}
	if r.Code == 0 || r.Code == 20000000 || r.Code == CodeSuccess {
		return nil
	}
	msg := strings.TrimSpace(r.Message)
	if msg == "" {
		msg = "audio generation request failed"
	}
	return &Error{Code: r.Code, Message: msg, ReqID: r.ReqID, TraceID: r.TraceID, LogID: r.LogID}
}
