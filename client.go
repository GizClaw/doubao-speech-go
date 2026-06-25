package doubaospeech

import (
	"net/http"
	"reflect"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/auth"
	"github.com/GizClaw/doubao-speech-go/internal/transport"
)

const (
	defaultBaseURL = "https://openspeech.bytedance.com"
	defaultWSURL   = "wss://openspeech.bytedance.com"
	defaultTimeout = 30 * time.Second
)

// V2/V3 Resource IDs.
const (
	ResourceTTSV1        = "seed-tts-1.0"
	ResourceTTSV1Concurr = "seed-tts-1.0-concurr"
	ResourceTTSV2        = "seed-tts-2.0"
	ResourceTTSV2Concurr = "seed-tts-2.0-concurr"
	ResourceVoiceCloneV1 = "seed-icl-1.0"
	ResourceVoiceCloneV2 = "seed-icl-2.0"

	ResourceASRStream   = "volc.bigasr.sauc.duration"
	ResourceASRStreamV2 = "volc.seedasr.sauc.duration"
	ResourceASRFile     = "volc.bigasr.auc.duration"

	ResourceRealtime     = "volc.speech.dialog"
	ResourcePodcast      = "volc.service_type.10050"
	ResourceTranslation  = "volc.megatts.simt"
	ResourceASTTranslate = "volc.service_type.10053"
)

// Client is the SDK entry point.
//
// In this migration stage, ASR V2, TTS V2 WS, Voice Clone, and Realtime are implemented.
type Client struct {
	// ASR V2 streaming recognition.
	ASR   *ASRServiceV2
	ASRV2 *ASRServiceV2

	// Voice cloning.
	VoiceClone *VoiceCloneService

	// Realtime dialogue.
	Realtime *RealtimeService

	// Realtime duplex dialogue.
	RealtimeDuplex *RealtimeDuplexService

	// AST realtime translation.
	ASTTranslate *ASTTranslateService
	AST          *ASTTranslateService

	// TTS V2 WebSocket synthesis.
	TTS   *TTSServiceV2
	TTSV2 *TTSServiceV2

	config *clientConfig
}

type clientConfig struct {
	appID  string
	apiKey string

	cluster    string
	resourceID string

	baseURL    string
	wsURL      string
	httpClient transport.HTTPDoer
	timeout    time.Duration
	userID     string
}

// Option configures Client.
type Option func(*clientConfig)

// NewClient creates an SDK client.
func NewClient(appID string, opts ...Option) *Client {
	cfg := &clientConfig{
		appID:   appID,
		baseURL: defaultBaseURL,
		wsURL:   defaultWSURL,
		timeout: defaultTimeout,
		userID:  "default_user",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if isNilHTTPDoer(cfg.httpClient) {
		cfg.httpClient = &http.Client{Timeout: cfg.timeout}
	}

	c := &Client{config: cfg}
	asrV2 := newASRServiceV2(c)
	ttsV2 := newTTSServiceV2(c)
	voiceClone := newVoiceCloneService(c)
	c.ASR = asrV2
	c.ASRV2 = asrV2
	c.VoiceClone = voiceClone
	c.Realtime = newRealtimeService(c)
	c.RealtimeDuplex = newRealtimeDuplexService(c)
	astTranslate := newASTTranslateService(c)
	c.ASTTranslate = astTranslate
	c.AST = astTranslate
	c.TTS = ttsV2
	c.TTSV2 = ttsV2

	return c
}

// WithAPIKey sets X-Api-Key authentication.
func WithAPIKey(apiKey string) Option {
	return func(c *clientConfig) {
		c.apiKey = apiKey
	}
}

// WithResourceID sets the default resource_id.
func WithResourceID(resourceID string) Option {
	return func(c *clientConfig) {
		c.resourceID = resourceID
	}
}

// WithCluster sets the V1 cluster (kept for backward compatibility).
func WithCluster(cluster string) Option {
	return func(c *clientConfig) {
		c.cluster = cluster
	}
}

// WithBaseURL sets the HTTP base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithWebSocketURL sets the WebSocket base URL.
func WithWebSocketURL(url string) Option {
	return func(c *clientConfig) {
		c.wsURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		if client == nil {
			return
		}
		c.httpClient = client
	}
}

// WithHTTPTransport sets a custom HTTP transport doer.
func WithHTTPTransport(doer transport.HTTPDoer) Option {
	return func(c *clientConfig) {
		if isNilHTTPDoer(doer) {
			return
		}
		c.httpClient = doer
	}
}

func isNilHTTPDoer(doer transport.HTTPDoer) bool {
	if doer == nil {
		return true
	}

	v := reflect.ValueOf(doer)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// WithTimeout sets request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = timeout
	}
}

// WithUserID sets user.uid.
func WithUserID(userID string) Option {
	return func(c *clientConfig) {
		c.userID = userID
	}
}

func (c *Client) authCredentials() auth.Credentials {
	return auth.Credentials{
		AppID:             c.config.appID,
		APIKey:            c.config.apiKey,
		DefaultResourceID: c.config.resourceID,
	}
}

func (c *Client) resolveResourceID(explicit string, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if c.config.resourceID != "" {
		return c.config.resourceID
	}
	return fallback
}
