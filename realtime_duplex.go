package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"
	"sync"

	"github.com/GizClaw/doubao-speech-go/internal/transport"
	"github.com/GizClaw/doubao-speech-go/internal/util"
	"github.com/gorilla/websocket"
)

const realtimeDuplexEndpointPath = "/api/v3/duplex/realtime/dialogue"

// RealtimeDuplexService provides full-duplex realtime dialogue operations.
type RealtimeDuplexService struct {
	client *Client
	dialer transport.WSDialer
}

func newRealtimeDuplexService(c *Client) *RealtimeDuplexService {
	return &RealtimeDuplexService{
		client: c,
		dialer: transport.NewGorillaDialer(nil),
	}
}

// Connect is a convenience alias for OpenSession.
func (s *RealtimeDuplexService) Connect(ctx context.Context, cfg *RealtimeDuplexConfig) (*RealtimeDuplexSession, error) {
	return s.OpenSession(ctx, cfg)
}

// OpenSession opens a realtime duplex WebSocket session and waits for session.created.
func (s *RealtimeDuplexService) OpenSession(ctx context.Context, cfg *RealtimeDuplexConfig) (*RealtimeDuplexSession, error) {
	normalized := normalizeRealtimeDuplexConfig(cfg)
	sessionID := normalized.Session.ID
	if sessionID == "" {
		sessionID = util.NewReqID("duplex")
		normalized.Session.ID = sessionID
	}

	endpoint := strings.TrimRight(s.client.config.wsURL, "/") + realtimeDuplexEndpointPath
	headers := buildRealtimeDuplexHeaders(s.client.config)
	conn, resp, err := s.dialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		return nil, wsConnectError(err, resp, responseMetadata{ReqID: sessionID, ConnectID: sessionID})
	}

	session := &RealtimeDuplexSession{
		conn:      conn,
		service:   s,
		sessionID: sessionID,
		logID:     responseHeader(resp, "X-Tt-Logid"),
		closed:    make(chan struct{}),
	}

	if err := session.sendSessionEvent(ctx, RealtimeDuplexEventSessionCreate, normalized); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "send realtime duplex session.create")
	}

	for {
		evt, err := session.recvEvent(ctx)
		if err != nil {
			_ = session.Close()
			return nil, wrapError(err, "read realtime duplex session.created")
		}
		switch evt.Type {
		case RealtimeDuplexEventSessionCreated:
			if evt.SessionID != "" {
				session.sessionID = evt.SessionID
			}
			return session, nil
		case RealtimeDuplexEventSessionUpdated:
			continue
		default:
			_ = session.Close()
			return nil, fmt.Errorf("unexpected realtime duplex session response event: %s", evt.Type)
		}
	}
}

// RealtimeDuplexSession represents one full-duplex realtime dialogue session.
type RealtimeDuplexSession struct {
	conn    transport.WSConn
	service *RealtimeDuplexService

	sessionID string
	logID     string

	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	closed    chan struct{}
}

// SessionID returns the current server session id.
func (s *RealtimeDuplexSession) SessionID() string {
	return s.sessionID
}

// LogID returns the X-Tt-Logid value from the WebSocket handshake, if present.
func (s *RealtimeDuplexSession) LogID() string {
	return s.logID
}

// UpdateSession sends session.update with a new config.
func (s *RealtimeDuplexSession) UpdateSession(ctx context.Context, cfg RealtimeDuplexConfig) error {
	if cfg.Session.ID == "" {
		cfg.Session.ID = s.sessionID
	}
	return s.sendSessionEvent(ctx, RealtimeDuplexEventSessionUpdate, cfg)
}

// SendAudio appends one raw audio chunk to input_audio_buffer.
func (s *RealtimeDuplexSession) SendAudio(ctx context.Context, audio []byte) error {
	return s.sendJSONEvent(ctx, realtimeDuplexAudioAppendEvent{
		Type:  RealtimeDuplexEventInputAudioBufferAppend,
		Audio: base64.StdEncoding.EncodeToString(audio),
	})
}

// CommitAudio commits current input audio.
func (s *RealtimeDuplexSession) CommitAudio(ctx context.Context) error {
	return s.sendSimpleEvent(ctx, RealtimeDuplexEventInputAudioBufferCommit)
}

// AppendSpeechText appends one streaming speech-text fragment.
func (s *RealtimeDuplexSession) AppendSpeechText(ctx context.Context, req RealtimeDuplexSpeechTextRequest) error {
	return s.sendSpeechTextEvent(ctx, RealtimeDuplexEventSpeechTextBufferAppend, req)
}

// SendSpeechText sends one complete speech-text commit.
func (s *RealtimeDuplexSession) SendSpeechText(ctx context.Context, req RealtimeDuplexSpeechTextRequest) error {
	return s.CommitSpeechText(ctx, req)
}

// CommitSpeechText commits speech text for TTS.
func (s *RealtimeDuplexSession) CommitSpeechText(ctx context.Context, req RealtimeDuplexSpeechTextRequest) error {
	return s.sendSpeechTextEvent(ctx, RealtimeDuplexEventSpeechTextBufferCommit, req)
}

// AppendReplacementSpeechText appends replacement text to intervene in the response.
func (s *RealtimeDuplexSession) AppendReplacementSpeechText(ctx context.Context, req RealtimeDuplexSpeechTextRequest) error {
	return s.sendSpeechTextEvent(ctx, RealtimeDuplexEventSpeechTextBufferReplacementAppend, req)
}

// CommitReplacementSpeechText commits replacement text.
func (s *RealtimeDuplexSession) CommitReplacementSpeechText(ctx context.Context, req RealtimeDuplexSpeechTextRequest) error {
	return s.sendSpeechTextEvent(ctx, RealtimeDuplexEventSpeechTextBufferReplacementCommit, req)
}

// CancelResponse cancels the in-flight response.
func (s *RealtimeDuplexSession) CancelResponse(ctx context.Context) error {
	return s.sendSimpleEvent(ctx, RealtimeDuplexEventResponseCancel)
}

// CreateConversationItems creates conversation context items.
func (s *RealtimeDuplexSession) CreateConversationItems(ctx context.Context, items ...RealtimeDuplexConversationItem) error {
	return s.sendConversationItems(ctx, RealtimeDuplexEventConversationItemCreate, items...)
}

// UpdateConversationItems updates conversation context items.
func (s *RealtimeDuplexSession) UpdateConversationItems(ctx context.Context, items ...RealtimeDuplexConversationItem) error {
	return s.sendConversationItems(ctx, RealtimeDuplexEventConversationItemUpdate, items...)
}

// RetrieveConversationItems retrieves conversation context items.
func (s *RealtimeDuplexSession) RetrieveConversationItems(ctx context.Context, items ...RealtimeDuplexConversationItem) error {
	return s.sendConversationItems(ctx, RealtimeDuplexEventConversationItemRetrieve, items...)
}

// DeleteConversationItems deletes conversation context items.
func (s *RealtimeDuplexSession) DeleteConversationItems(ctx context.Context, items ...RealtimeDuplexConversationItem) error {
	return s.sendConversationItems(ctx, RealtimeDuplexEventConversationItemDelete, items...)
}

// SendFunctionCallOutputs returns function-call outputs to the service.
func (s *RealtimeDuplexSession) SendFunctionCallOutputs(ctx context.Context, outputs ...RealtimeDuplexFunctionCallOutput) error {
	items := make([]RealtimeDuplexConversationItem, 0, len(outputs))
	for _, output := range outputs {
		items = append(items, RealtimeDuplexConversationItem{
			Type:   "message",
			Role:   RealtimeDuplexRoleTool,
			CallID: output.CallID,
			Content: []RealtimeDuplexConversationContent{
				{Type: "input_text", Text: output.Output},
			},
		})
	}
	return s.CreateConversationItems(ctx, items...)
}

// RecvEvent receives and decodes the next server event.
func (s *RealtimeDuplexSession) RecvEvent(ctx context.Context) (*RealtimeDuplexEvent, error) {
	return s.recvEvent(ctx)
}

// Recv returns an iterator over server events.
func (s *RealtimeDuplexSession) Recv() iter.Seq2[*RealtimeDuplexEvent, error] {
	return func(yield func(*RealtimeDuplexEvent, error) bool) {
		for {
			evt, err := s.RecvEvent(context.Background())
			if !yield(evt, err) || err != nil {
				return
			}
		}
	}
}

// Close sends session.close once and closes the WebSocket connection.
func (s *RealtimeDuplexSession) Close() error {
	s.closeOnce.Do(func() {
		_ = s.sendSimpleEvent(context.Background(), RealtimeDuplexEventSessionClose)
		close(s.closed)
		s.closeErr = s.conn.Close()
	})
	return s.closeErr
}

func (s *RealtimeDuplexSession) sendSessionEvent(ctx context.Context, eventType string, cfg RealtimeDuplexConfig) error {
	return s.sendJSONEvent(ctx, realtimeDuplexSessionEvent{
		Type:      eventType,
		EventID:   util.NewReqID("duplex-event"),
		Session:   cfg.Session,
		Extension: cfg.Extension,
	})
}

func (s *RealtimeDuplexSession) sendSimpleEvent(ctx context.Context, eventType string) error {
	return s.sendJSONEvent(ctx, realtimeDuplexSimpleEvent{
		Type:    eventType,
		EventID: util.NewReqID("duplex-event"),
	})
}

func (s *RealtimeDuplexSession) sendSpeechTextEvent(ctx context.Context, eventType string, req RealtimeDuplexSpeechTextRequest) error {
	return s.sendJSONEvent(ctx, realtimeDuplexSpeechTextEvent{
		Type:      eventType,
		EventID:   firstNonEmpty(req.EventID, util.NewReqID("duplex-event")),
		SpeechID:  req.SpeechID,
		Text:      req.Text,
		TTSPrompt: req.TTSPrompt,
	})
}

func (s *RealtimeDuplexSession) sendConversationItems(ctx context.Context, eventType string, items ...RealtimeDuplexConversationItem) error {
	return s.sendJSONEvent(ctx, realtimeDuplexConversationItemsEvent{
		Type:    eventType,
		EventID: util.NewReqID("duplex-event"),
		Items:   items,
	})
}

func (s *RealtimeDuplexSession) sendJSONEvent(ctx context.Context, event any) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	select {
	case <-s.closed:
		return newAPIError(CodeServerError, "realtime duplex session is closed")
	default:
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return wrapError(err, "marshal realtime duplex event")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return wrapError(err, "send realtime duplex event")
	}
	return nil
}

func (s *RealtimeDuplexSession) recvEvent(ctx context.Context) (*RealtimeDuplexEvent, error) {
	msgType, payload, err := readWSMessageWithContext(ctx, s.conn)
	if err != nil {
		return nil, err
	}
	switch msgType {
	case websocket.TextMessage, websocket.BinaryMessage:
		return decodeRealtimeDuplexEvent(payload)
	default:
		return nil, fmt.Errorf("unsupported realtime duplex websocket message type: %d", msgType)
	}
}

func normalizeRealtimeDuplexConfig(cfg *RealtimeDuplexConfig) RealtimeDuplexConfig {
	base := DefaultRealtimeDuplexConfig()
	if cfg == nil {
		return base
	}
	if cfg.Session.ID != "" {
		base.Session.ID = cfg.Session.ID
	}
	if cfg.Session.Model != "" {
		base.Session.Model = cfg.Session.Model
	}
	if cfg.Session.Instructions != "" {
		base.Session.Instructions = cfg.Session.Instructions
	}
	if cfg.Session.Audio.Input.Format.Type != "" {
		base.Session.Audio.Input = cfg.Session.Audio.Input
	}
	if cfg.Session.Audio.Output.Format.Type != "" ||
		cfg.Session.Audio.Output.Format.Rate != 0 ||
		cfg.Session.Audio.Output.Voice != "" ||
		cfg.Session.Audio.Output.Speed != 0 ||
		cfg.Session.Audio.Output.Loudness != 0 {
		base.Session.Audio.Output = cfg.Session.Audio.Output
	}
	if len(cfg.Session.Tools) > 0 {
		base.Session.Tools = cfg.Session.Tools
	}
	base.Extension = cfg.Extension
	return base
}

func buildRealtimeDuplexHeaders(cfg *clientConfig) http.Header {
	headers := http.Header{}
	switch {
	case cfg.apiKey != "":
		headers.Set("X-Api-Key", cfg.apiKey)
	case cfg.accessToken != "":
		headers.Set("Authorization", "Bearer "+cfg.accessToken)
	case cfg.accessKey != "":
		headers.Set("X-Api-Key", cfg.accessKey)
	}
	return headers
}

func responseHeader(resp *http.Response, key string) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get(key)
}
