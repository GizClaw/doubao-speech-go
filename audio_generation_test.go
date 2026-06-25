package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAudioGenerationCreateSuccess(t *testing.T) {
	audio := []byte("fake wav bytes")
	var captured struct {
		Path      string
		Method    string
		Headers   http.Header
		RawBody   []byte
		Body      map[string]any
		RequestID string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Path = r.URL.Path
		captured.Method = r.Method
		captured.Headers = r.Header.Clone()
		captured.RequestID = r.Header.Get("X-Api-Request-Id")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		captured.RawBody = body
		if err := json.Unmarshal(body, &captured.Body); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		w.Header().Set("X-Tt-Logid", "log-audio")
		_, _ = io.WriteString(w, `{"code":20000000,"message":"ok","audio":"`+base64.StdEncoding.EncodeToString(audio)+`","duration":1.25,"original_duration":1.5,"url":"https://example.com/audio.wav","unknown":{"x":1}}`)
	}))
	defer server.Close()

	explicitFalse := false
	client := NewClient("app-test",
		WithBaseURL(server.URL),
		WithAPIKey("key-test"),
		WithResourceID(ResourceTTSV2),
	)

	resp, err := client.AudioGeneration.Create(context.Background(), &AudioGenerationCreateRequest{
		Model:      " " + ModelSeedAudio10 + " ",
		TextPrompt: "  Generate a short bright chime.  ",
		RequestID:  "req-audio-test",
		References: []AudioGenerationReference{
			{Speaker: " zh_female_xiaohe_uranus_bigtts "},
		},
		AudioConfig: &AudioGenerationAudioConfig{
			Format:       FormatWAV,
			SampleRate:   SampleRate24000,
			SpeechRate:   10,
			LoudnessRate: -5,
			PitchRate:    2,
		},
		Watermark: &AudioGenerationWatermark{
			AIGCWatermark: &explicitFalse,
			AIGCMetadata: &AudioGenerationAIGCMeta{
				Enable:          &explicitFalse,
				ContentProducer: "  test-producer  ",
				ProduceID:       " produce-1 ",
			},
		},
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if captured.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", captured.Method)
	}
	if captured.Path != audioGenerationCreatePath {
		t.Fatalf("path = %q, want %q", captured.Path, audioGenerationCreatePath)
	}
	if got := captured.Headers.Get("X-Api-Key"); got != "key-test" {
		t.Fatalf("X-Api-Key = %q, want key-test", got)
	}
	if got := captured.Headers.Get("X-Api-App-Id"); got != "app-test" {
		t.Fatalf("X-Api-App-Id = %q, want app-test", got)
	}
	if got := captured.Headers.Get("X-Api-Access-Key"); got != "" {
		t.Fatalf("X-Api-Access-Key = %q, want empty", got)
	}
	if got := captured.Headers.Get("X-Api-Resource-Id"); got != "" {
		t.Fatalf("X-Api-Resource-Id = %q, want empty for audio generation", got)
	}
	if captured.RequestID != "req-audio-test" {
		t.Fatalf("X-Api-Request-Id = %q, want req-audio-test", captured.RequestID)
	}

	if got := captured.Body["model"]; got != ModelSeedAudio10 {
		t.Fatalf("model = %v, want %s", got, ModelSeedAudio10)
	}
	if got := captured.Body["text_prompt"]; got != "Generate a short bright chime." {
		t.Fatalf("text_prompt = %v", got)
	}
	refs := captured.Body["references"].([]any)
	ref := refs[0].(map[string]any)
	if got := ref["speaker"]; got != "zh_female_xiaohe_uranus_bigtts" {
		t.Fatalf("speaker = %v", got)
	}
	watermark := captured.Body["watermark"].(map[string]any)
	if got := watermark["aigc_watermark"]; got != false {
		t.Fatalf("aigc_watermark = %v, want explicit false", got)
	}
	meta := watermark["aigc_metadata"].(map[string]any)
	if got := meta["enable"]; got != false {
		t.Fatalf("metadata enable = %v, want explicit false", got)
	}
	if got := meta["content_producer"]; got != "test-producer" {
		t.Fatalf("content_producer = %v", got)
	}

	if string(resp.Audio) != string(audio) {
		t.Fatalf("decoded audio = %q, want %q", string(resp.Audio), string(audio))
	}
	if resp.URL != "https://example.com/audio.wav" {
		t.Fatalf("url = %q", resp.URL)
	}
	if resp.Duration != 1.25 || resp.OriginalDuration != 1.5 {
		t.Fatalf("durations = %v/%v", resp.Duration, resp.OriginalDuration)
	}
	if resp.ReqID != "req-audio-test" {
		t.Fatalf("reqid = %q, want generated request id fallback", resp.ReqID)
	}
	if resp.LogID != "log-audio" {
		t.Fatalf("log id = %q, want log-audio", resp.LogID)
	}
	if len(resp.Extra["unknown"]) == 0 {
		t.Fatalf("unknown response field was not preserved")
	}
}

func TestAudioGenerationCreateGeneratesRequestID(t *testing.T) {
	var requestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = r.Header.Get("X-Api-Request-Id")
		_, _ = io.WriteString(w, `{"code":20000000,"audio":"`+base64.StdEncoding.EncodeToString([]byte("ok"))+`"}`)
	}))
	defer server.Close()

	client := NewClient("app-test", WithBaseURL(server.URL), WithAPIKey("key-test"))
	resp, err := client.AudioGeneration.Create(context.Background(), &AudioGenerationCreateRequest{
		Model:      ModelSeedAudio10,
		TextPrompt: "Generate a tiny click.",
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if requestID == "" || !strings.HasPrefix(requestID, "audio-") {
		t.Fatalf("request id = %q, want generated audio-* id", requestID)
	}
	if resp.ReqID != requestID {
		t.Fatalf("response reqid = %q, want %q", resp.ReqID, requestID)
	}
}

func TestAudioGenerationCreateResponseMetadataAliases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"code":20000000,"audio":"`+base64.StdEncoding.EncodeToString([]byte("ok"))+`","request_id":"req-body","logid":"log-body"}`)
	}))
	defer server.Close()

	client := NewClient("app-test", WithBaseURL(server.URL), WithAPIKey("key-test"))
	resp, err := client.AudioGeneration.Create(context.Background(), &AudioGenerationCreateRequest{
		Model:      ModelSeedAudio10,
		TextPrompt: "Generate a tiny click.",
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if resp.ReqID != "req-body" {
		t.Fatalf("reqid = %q, want req-body", resp.ReqID)
	}
	if resp.LogID != "log-body" {
		t.Fatalf("logid = %q, want log-body", resp.LogID)
	}
}

func TestAudioGenerationCreateBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Tt-Logid", "log-business")
		_, _ = io.WriteString(w, `{"code":45000001,"message":"bad prompt"}`)
	}))
	defer server.Close()

	client := NewClient("app-test", WithBaseURL(server.URL), WithAPIKey("key-test"))
	_, err := client.AudioGeneration.Create(context.Background(), &AudioGenerationCreateRequest{
		Model:      ModelSeedAudio10,
		TextPrompt: "Generate audio.",
		RequestID:  "req-business",
	})
	if err == nil {
		t.Fatalf("expected business error")
	}
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != 45000001 || apiErr.Message != "bad prompt" {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.ReqID != "req-business" || apiErr.LogID != "log-business" {
		t.Fatalf("metadata = reqid %q logid %q", apiErr.ReqID, apiErr.LogID)
	}
}

func TestAudioGenerationCreateHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Tt-Logid", "log-http")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"code":3002,"message":"forbidden","request_id":"req-http"}`)
	}))
	defer server.Close()

	client := NewClient("app-test", WithBaseURL(server.URL), WithAPIKey("key-test"))
	_, err := client.AudioGeneration.Create(context.Background(), &AudioGenerationCreateRequest{
		Model:      ModelSeedAudio10,
		TextPrompt: "Generate audio.",
	})
	if err == nil {
		t.Fatalf("expected http error")
	}
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.HTTPStatus != http.StatusForbidden || apiErr.Code != 3002 {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.ReqID != "req-http" || apiErr.LogID != "log-http" {
		t.Fatalf("metadata = reqid %q logid %q", apiErr.ReqID, apiErr.LogID)
	}
}

func TestAudioGenerationCreateMalformedAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"code":20000000,"audio":"not base64 ***"}`)
	}))
	defer server.Close()

	client := NewClient("app-test", WithBaseURL(server.URL), WithAPIKey("key-test"))
	_, err := client.AudioGeneration.Create(context.Background(), &AudioGenerationCreateRequest{
		Model:      ModelSeedAudio10,
		TextPrompt: "Generate audio.",
	})
	if err == nil {
		t.Fatalf("expected malformed audio error")
	}
	if !strings.Contains(err.Error(), "decode audio generation audio") {
		t.Fatalf("error = %v", err)
	}
}

func TestAudioGenerationCreateContextCanceled(t *testing.T) {
	client := NewClient("app-test",
		WithBaseURL("https://example.com"),
		WithAPIKey("key-test"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.AudioGeneration.Create(ctx, &AudioGenerationCreateRequest{
		Model:      ModelSeedAudio10,
		TextPrompt: "Generate audio.",
	})
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
}

func TestNormalizeAudioGenerationCreateRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *AudioGenerationCreateRequest
		want string
	}{
		{
			name: "nil",
			req:  nil,
			want: "request is nil",
		},
		{
			name: "missing model",
			req:  &AudioGenerationCreateRequest{TextPrompt: "x"},
			want: "model is required",
		},
		{
			name: "unsupported model",
			req:  &AudioGenerationCreateRequest{Model: "seed-audio-next", TextPrompt: "x"},
			want: "model must be " + ModelSeedAudio10,
		},
		{
			name: "missing prompt",
			req:  &AudioGenerationCreateRequest{Model: ModelSeedAudio10},
			want: "text_prompt is required",
		},
		{
			name: "mixed reference kinds",
			req: &AudioGenerationCreateRequest{
				Model:      ModelSeedAudio10,
				TextPrompt: "x",
				References: []AudioGenerationReference{
					{Speaker: "speaker"},
					{ImageURL: "https://example.com/image.png"},
				},
			},
			want: "audio and image references cannot be mixed",
		},
		{
			name: "too many audio refs",
			req: &AudioGenerationCreateRequest{
				Model:      ModelSeedAudio10,
				TextPrompt: "x",
				References: []AudioGenerationReference{
					{Speaker: "a"},
					{Speaker: "b"},
					{Speaker: "c"},
					{Speaker: "d"},
				},
			},
			want: "at most 3 audio references",
		},
		{
			name: "too many image refs",
			req: &AudioGenerationCreateRequest{
				Model:      ModelSeedAudio10,
				TextPrompt: "x",
				References: []AudioGenerationReference{
					{ImageURL: "https://example.com/1.png"},
					{ImageURL: "https://example.com/2.png"},
				},
			},
			want: "at most 1 image reference",
		},
		{
			name: "multiple fields in one audio ref",
			req: &AudioGenerationCreateRequest{
				Model:      ModelSeedAudio10,
				TextPrompt: "x",
				References: []AudioGenerationReference{
					{Speaker: "speaker", AudioURL: "https://example.com/ref.wav"},
				},
			},
			want: "only one of speaker, audio_data, or audio_url",
		},
		{
			name: "bad format",
			req: &AudioGenerationCreateRequest{
				Model:       ModelSeedAudio10,
				TextPrompt:  "x",
				AudioConfig: &AudioGenerationAudioConfig{Format: "flac"},
			},
			want: "unsupported audio format",
		},
		{
			name: "bad speech rate",
			req: &AudioGenerationCreateRequest{
				Model:       ModelSeedAudio10,
				TextPrompt:  "x",
				AudioConfig: &AudioGenerationAudioConfig{SpeechRate: 101},
			},
			want: "speech_rate must be in range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeAudioGenerationCreateRequest(tt.req)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want contains %q", err, tt.want)
			}
		})
	}
}
