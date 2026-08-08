package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/GizClaw/doubao-speech-go/internal/auth"
	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/GizClaw/doubao-speech-go/internal/transport"
	"github.com/GizClaw/doubao-speech-go/internal/util"
	"github.com/gorilla/websocket"
)

const (
	realtimeEndpointPath               = "/api/v3/realtime/dialogue"
	realtimeStartSessionEvent    int32 = 100
	realtimeFinishSessionEvent   int32 = 102
	realtimeTaskAudioEvent       int32 = 200
	realtimeUpdateConfigEvent    int32 = 201
	realtimeSayHelloEvent        int32 = 300
	realtimeEndASREvent          int32 = 400
	realtimeTTSTextEvent         int32 = 500
	realtimeUserTextEvent        int32 = 501
	realtimeRAGTextEvent         int32 = 502
	realtimeConversationCreate   int32 = 510
	realtimeConversationUpdate   int32 = 511
	realtimeConversationRetrieve int32 = 512
	realtimeConversationTruncate int32 = 513
	realtimeConversationDelete   int32 = 514
	realtimeClientInterrupt      int32 = 515

	defaultRealtimeEventBuffer         = 64
	defaultRealtimeBackpressureTimeout = 2 * time.Second
	defaultRealtimeCloseWaitTimeout    = 2 * time.Second
)

// RealtimeService provides real-time dialogue operations.
type RealtimeService struct {
	client *Client
	dialer transport.WSDialer
}

func newRealtimeService(c *Client) *RealtimeService {
	return &RealtimeService{
		client: c,
		dialer: transport.NewGorillaDialer(nil),
	}
}

// Dial opens a realtime websocket connection and completes StartConnection handshake.
func (s *RealtimeService) Dial(ctx context.Context) (*RealtimeConnection, error) {
	connectReqID := util.NewReqID("rt")
	resourceID := s.client.resolveResourceID("", ResourceRealtime)

	headers := auth.BuildV2WSHeaders(s.client.authCredentials(), resourceID, connectReqID)
	headers.Set("X-Api-Request-Id", connectReqID)

	endpoint := strings.TrimRight(s.client.config.wsURL, "/") + realtimeEndpointPath
	conn, resp, err := s.dialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		return nil, wsConnectError(err, resp, responseMetadata{ReqID: connectReqID, ConnectID: connectReqID})
	}

	rtConn := &RealtimeConnection{
		conn:      conn,
		service:   s,
		connectID: connectReqID,
		closed:    make(chan struct{}),
	}

	if err := rtConn.sendConnectionStart(ctx); err != nil {
		_ = rtConn.Close()
		return nil, wrapError(err, "send start connection")
	}

	frame, err := rtConn.readFrameWithContext(ctx)
	if err != nil {
		_ = rtConn.Close()
		return nil, wrapError(err, "read connection response")
	}
	if frame.MessageType == protocol.MessageTypeError {
		_ = rtConn.Close()
		return nil, wrapError(
			withErrorMetadata(parseWSErrorPayload(frame.Payload, frame.ErrorCode), responseMetadata{ReqID: connectReqID, ConnectID: connectReqID}),
			"connection failed",
		)
	}
	if frame.HasEvent && frame.Event == int32(EventConnectionFailed) {
		_ = rtConn.Close()
		return nil, wrapError(
			withErrorMetadata(realtimePayloadError(frame.Payload, CodeServerError, "connection failed"), responseMetadata{ReqID: connectReqID, ConnectID: connectReqID}),
			"connection failed",
		)
	}
	if !frame.HasEvent || frame.Event != int32(EventConnectionStarted) {
		_ = rtConn.Close()
		return nil, fmt.Errorf("unexpected connection response event: %d", frame.Event)
	}
	if frame.ConnectID != "" {
		rtConn.connectID = frame.ConnectID
	}

	return rtConn, nil
}

// Connect is a convenience method for Dial + StartSession.
func (s *RealtimeService) Connect(ctx context.Context, cfg *RealtimeConfig) (*RealtimeSession, error) {
	conn, err := s.Dial(ctx)
	if err != nil {
		return nil, err
	}

	session, err := conn.StartSession(ctx, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return session, nil
}

// OpenSession is a compatibility alias for Connect.
func (s *RealtimeService) OpenSession(ctx context.Context, cfg *RealtimeConfig) (*RealtimeSession, error) {
	return s.Connect(ctx, cfg)
}

// RealtimeConnection represents an established realtime websocket connection.
type RealtimeConnection struct {
	conn    transport.WSConn
	service *RealtimeService

	connectID string

	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	closed    chan struct{}
}

// StartSession starts one realtime session on current connection.
func (c *RealtimeConnection) StartSession(ctx context.Context, cfg *RealtimeConfig) (*RealtimeSession, error) {
	normalized, err := normalizeRealtimeConfig(cfg)
	if err != nil {
		return nil, err
	}

	sessionID := util.NewReqID("session")
	startPayload, err := buildRealtimeStartPayload(normalized)
	if err != nil {
		return nil, wrapError(err, "marshal start session payload")
	}

	packet, err := protocol.BuildFullClientJSONWithEvent(realtimeStartSessionEvent, sessionID, startPayload)
	if err != nil {
		return nil, wrapError(err, "encode start session event")
	}
	if err := c.writeBinary(ctx, packet); err != nil {
		return nil, wrapError(err, "send start session")
	}

	frame, err := c.readFrameWithContext(ctx)
	if err != nil {
		return nil, wrapError(err, "read start session response")
	}
	if frame.MessageType == protocol.MessageTypeError {
		return nil, wrapError(
			withErrorMetadata(parseWSErrorPayload(frame.Payload, frame.ErrorCode), responseMetadata{ReqID: sessionID, ConnectID: c.connectID}),
			"start session failed",
		)
	}
	if frame.HasEvent && frame.Event == int32(EventSessionFailed) {
		return nil, wrapError(
			withErrorMetadata(realtimePayloadError(frame.Payload, CodeServerError, "session failed"), responseMetadata{ReqID: sessionID, ConnectID: c.connectID}),
			"start session failed",
		)
	}
	if !frame.HasEvent || frame.Event != int32(EventSessionStarted) {
		return nil, fmt.Errorf("unexpected session response event: %d", frame.Event)
	}
	if frame.SessionID != "" {
		sessionID = frame.SessionID
	}

	started := &RealtimeEvent{Type: EventSessionStarted, Payload: copyBytes(frame.Payload)}
	decodeEventPayload(started)

	session := &RealtimeSession{
		conn:      c,
		sessionID: sessionID,
		dialogID:  started.DialogID,
		model:     normalized.Model,
		inputMode: normalized.InputMode,
		eventCh:   make(chan *RealtimeEvent, normalized.EventBuffer),
		errCh:     make(chan error, 1),
		closed:    make(chan struct{}),
		recvDone:  make(chan struct{}),

		backpressureTimeout: normalized.BackpressureTimeout,

		history: cloneConversationHistory(normalized.History),
		prompt:  clonePromptConfig(normalized.Prompt),
		props:   cloneGenerationProps(normalized.Props),

		conversationTruncateEnabled: normalized.Dialog.Extra != nil && boolValue(normalized.Dialog.Extra.EnableConversationTruncate),
	}

	go session.receiveLoop()

	return session, nil
}

// Close closes websocket connection.
func (c *RealtimeConnection) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

func (c *RealtimeConnection) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *RealtimeConnection) writeBinary(ctx context.Context, packet []byte) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.isClosed() {
		return newAPIError(CodeServerError, "realtime connection already closed")
	}

	return c.conn.WriteMessage(websocket.BinaryMessage, packet)
}

func (c *RealtimeConnection) sendConnectionStart(ctx context.Context) error {
	packet, err := protocol.BuildFullClientJSONWithEvent(protocol.EventStartConnection, "", []byte("{}"))
	if err != nil {
		return wrapError(err, "encode start connection event")
	}
	return c.writeBinary(ctx, packet)
}

func (c *RealtimeConnection) sendConnectionFinish(ctx context.Context) error {
	packet, err := protocol.BuildFullClientJSONWithEvent(protocol.EventFinishConnection, "", []byte("{}"))
	if err != nil {
		return wrapError(err, "encode finish connection event")
	}
	return c.writeBinary(ctx, packet)
}

func (c *RealtimeConnection) readFrameWithContext(ctx context.Context) (*protocol.ParsedFrame, error) {
	msgType, payload, err := readWSMessageWithContext(ctx, c.conn)
	if err != nil {
		return nil, err
	}

	switch msgType {
	case websocket.BinaryMessage:
		frame, err := protocol.ParseServerFrame(payload)
		if err != nil {
			return nil, wrapError(err, "parse websocket binary frame")
		}
		return frame, nil
	case websocket.TextMessage:
		return nil, withErrorMetadata(parseWSErrorPayload(payload, 0), responseMetadata{ReqID: c.connectID, ConnectID: c.connectID})
	default:
		return nil, fmt.Errorf("unsupported websocket message type: %d", msgType)
	}
}

// RealtimeSession represents one realtime dialogue session.
type RealtimeSession struct {
	conn      *RealtimeConnection
	sessionID string
	dialogID  string
	model     RealtimeModelVersion
	inputMode RealtimeInputMode

	conversationTruncateEnabled bool

	eventCh  chan *RealtimeEvent
	errCh    chan error
	closed   chan struct{}
	recvDone chan struct{}

	backpressureTimeout time.Duration

	stateMu sync.RWMutex
	history []RealtimeConversationMessage
	prompt  RealtimePromptConfig
	props   RealtimeGenerationProps

	turnMu              sync.Mutex
	turnFinalDelivered  bool
	audioTurnInProgress bool

	recvMu   sync.Mutex
	recvBusy bool

	errOnce   sync.Once
	closeOnce sync.Once
	closeErr  error
}

// SessionID returns current session ID.
func (s *RealtimeSession) SessionID() string {
	return s.sessionID
}

// DialogID returns the dialogue identifier negotiated by SessionStarted.
func (s *RealtimeSession) DialogID() string {
	return s.dialogID
}

// SendAudio sends one audio chunk (event=200).
func (s *RealtimeSession) SendAudio(ctx context.Context, audio []byte) error {
	if len(audio) == 0 {
		return newAPIError(CodeParamError, "audio payload is empty")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	s.beginAudioTurn()

	packet, err := protocol.BuildAudioOnlyWithEvent(realtimeTaskAudioEvent, s.sessionID, audio)
	if err != nil {
		return wrapError(err, "encode audio event")
	}

	return s.conn.writeBinary(ctx, packet)
}

// SendText sends user text (event=501).
func (s *RealtimeSession) SendText(ctx context.Context, text string) error {
	return s.SendUserMessage(ctx, text)
}

// SendUserMessage sends one user text with current history/prompt/props snapshot.
func (s *RealtimeSession) SendUserMessage(ctx context.Context, text string) error {
	content := strings.TrimSpace(text)
	if content == "" {
		return newAPIError(CodeParamError, "text is empty")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	s.resetTurnFinalState()

	history, prompt, props := s.snapshotSessionState()
	payload := map[string]any{
		"content": content,
	}
	if len(history) > 0 {
		payload["history"] = history
	}
	if prompt.System != "" || len(prompt.Variables) > 0 {
		payload["prompt"] = prompt
	}
	if hasRealtimeProps(props) {
		payload["props"] = props
	}

	if err := s.sendJSONEvent(ctx, realtimeUserTextEvent, payload); err != nil {
		return err
	}

	s.appendHistory(RealtimeConversationMessage{Role: "user", Content: content})
	return nil
}

// SayHello sends SayHello event (event=300).
func (s *RealtimeSession) SayHello(ctx context.Context, content string) error {
	if strings.TrimSpace(content) == "" {
		return newAPIError(CodeParamError, "hello content is empty")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	s.resetTurnFinalState()
	return s.sendJSONEvent(ctx, realtimeSayHelloEvent, map[string]any{"content": content})
}

// EndASR signals the end of client-side audio input in push-to-talk mode (event=400).
func (s *RealtimeSession) EndASR(ctx context.Context) error {
	if s.inputMode != RealtimeInputModePushToTalk {
		return newAPIError(CodeParamError, "EndASR requires push_to_talk input mode")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeEndASREvent, map[string]any{})
}

// Interrupt interrupts current generation (event=515).
func (s *RealtimeSession) Interrupt(ctx context.Context) error {
	if s.inputMode != RealtimeInputModePushToTalk {
		return newAPIError(CodeParamError, "ClientInterrupt requires push_to_talk input mode")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeClientInterrupt, map[string]any{})
}

// UpdateConfig sends a full-replacement UpdateConfig request (event=201).
func (s *RealtimeSession) UpdateConfig(ctx context.Context, cfg RealtimeUpdateConfig) error {
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	payload, err := buildRealtimeUpdateConfigPayload(cfg, s.model)
	if err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeUpdateConfigEvent, payload)
}

// SendRAGText sends external RAG text to the model (event=502).
func (s *RealtimeSession) SendRAGText(ctx context.Context, externalRAG string) error {
	if strings.TrimSpace(externalRAG) == "" {
		return newAPIError(CodeParamError, "external_rag is empty")
	}
	if utf8.RuneCountInString(externalRAG) > 4000 {
		return newAPIError(CodeParamError, "external_rag must be at most 4000 characters")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	s.resetTurnFinalState()
	return s.sendJSONEvent(ctx, realtimeRAGTextEvent, map[string]any{"external_rag": externalRAG})
}

// CreateConversationItems appends server-side conversation context items (event=510).
func (s *RealtimeSession) CreateConversationItems(ctx context.Context, items ...RealtimeConversationItem) error {
	payloadItems, err := buildRealtimeConversationCreateItems(items)
	if err != nil {
		return err
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeConversationCreate, map[string]any{"items": payloadItems})
}

// UpdateConversationItems updates server-side conversation context items (event=511).
func (s *RealtimeSession) UpdateConversationItems(ctx context.Context, items ...RealtimeConversationItem) error {
	payloadItems, err := buildRealtimeConversationUpdateItems(items)
	if err != nil {
		return err
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeConversationUpdate, map[string]any{"items": payloadItems})
}

// RetrieveConversationItems retrieves server-side conversation context (event=512).
// Call with no item IDs to retrieve the latest complete context.
func (s *RealtimeSession) RetrieveConversationItems(ctx context.Context, itemIDs ...string) error {
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	items := make([]map[string]string, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" {
			items = append(items, map[string]string{"item_id": itemID})
		}
	}
	if len(items) == 0 {
		return s.sendJSONEvent(ctx, realtimeConversationRetrieve, map[string]any{})
	}
	return s.sendJSONEvent(ctx, realtimeConversationRetrieve, map[string]any{"items": items})
}

// TruncateConversationItem truncates one server-side context item by played audio duration (event=513).
func (s *RealtimeSession) TruncateConversationItem(ctx context.Context, itemID string, audioEndMS int) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return newAPIError(CodeParamError, "item_id is empty")
	}
	if audioEndMS < 0 {
		return newAPIError(CodeParamError, "audio_end_ms must be >= 0")
	}
	if !s.conversationTruncateEnabled {
		return newAPIError(CodeParamError, "ConversationTruncate requires enable_conversation_truncate")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeConversationTruncate, map[string]any{
		"item_id":      itemID,
		"audio_end_ms": audioEndMS,
	})
}

// DeleteConversationItems deletes server-side conversation context items (event=514).
func (s *RealtimeSession) DeleteConversationItems(ctx context.Context, itemIDs ...string) error {
	if len(itemIDs) == 0 {
		return newAPIError(CodeParamError, "item_ids are empty")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	items := make([]map[string]string, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" {
			items = append(items, map[string]string{"item_id": itemID})
		}
	}
	if len(items) == 0 {
		return newAPIError(CodeParamError, "item_ids are empty")
	}
	return s.sendJSONEvent(ctx, realtimeConversationDelete, map[string]any{"items": items})
}

// FinishSession ends the current session while leaving the websocket connection reusable (event=102).
func (s *RealtimeSession) FinishSession(ctx context.Context) error {
	if err := s.guardSend(ctx); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeFinishSessionEvent, map[string]any{})
}

// SendTTSText sends one complete caller-provided TTS text transaction (event=500).
func (s *RealtimeSession) SendTTSText(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return newAPIError(CodeParamError, "tts text is empty")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	s.resetTurnFinalState()
	if err := s.sendJSONEvent(ctx, realtimeTTSTextEvent, map[string]any{
		"start":   true,
		"content": "",
		"end":     false,
	}); err != nil {
		return err
	}
	if err := s.sendJSONEvent(ctx, realtimeTTSTextEvent, map[string]any{
		"start":   false,
		"content": text,
		"end":     false,
	}); err != nil {
		return err
	}
	return s.sendJSONEvent(ctx, realtimeTTSTextEvent, map[string]any{
		"start":   false,
		"content": "",
		"end":     true,
	})
}

// UpdateHistory replaces the whole local history snapshot used by future turns.
func (s *RealtimeSession) UpdateHistory(history []RealtimeConversationMessage) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.history = cloneConversationHistory(history)
}

// ReplaceHistory replaces one item in local history by index.
func (s *RealtimeSession) ReplaceHistory(index int, message RealtimeConversationMessage) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if index < 0 || index >= len(s.history) {
		return newAPIError(CodeParamError, "history index out of range")
	}
	s.history[index] = message
	return nil
}

// UpdatePrompt replaces current prompt config used by future turns.
func (s *RealtimeSession) UpdatePrompt(prompt RealtimePromptConfig) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.prompt = clonePromptConfig(prompt)
}

// UpdateProps replaces current generation props used by future turns.
func (s *RealtimeSession) UpdateProps(props RealtimeGenerationProps) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.props = cloneGenerationProps(props)
}

// Recv returns a streaming iterator. Concurrent Recv is not supported.
func (s *RealtimeSession) Recv() iter.Seq2[*RealtimeEvent, error] {
	return func(yield func(*RealtimeEvent, error) bool) {
		if err := s.beginRecv(); err != nil {
			yield(nil, err)
			return
		}
		defer s.endRecv()

		events := s.eventCh
		errs := s.errCh

		for events != nil || errs != nil {
			select {
			case evt, ok := <-events:
				if !ok {
					events = nil
					continue
				}
				if !yield(evt, nil) {
					return
				}
			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}
				yield(nil, err)
				return
			}
		}
	}
}

// RecvEvent receives one event. Concurrent Recv/RecvEvent is not supported.
func (s *RealtimeSession) RecvEvent(ctx context.Context) (*RealtimeEvent, error) {
	if err := s.beginRecv(); err != nil {
		return nil, err
	}
	defer s.endRecv()

	events := s.eventCh
	errs := s.errCh

	for events != nil || errs != nil {
		select {
		case evt, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			return evt, nil
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, nil
}

// Close closes current session. It is idempotent.
func (s *RealtimeSession) Close() error {
	s.closeOnce.Do(func() {
		// Best-effort finish signals.
		_ = s.sendJSONEvent(context.Background(), realtimeFinishSessionEvent, map[string]any{})
		_ = s.conn.sendConnectionFinish(context.Background())

		close(s.closed)
		s.closeErr = s.conn.Close()

		select {
		case <-s.recvDone:
		case <-time.After(defaultRealtimeCloseWaitTimeout):
		}
	})

	return s.closeErr
}

func (s *RealtimeSession) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

func (s *RealtimeSession) guardSend(ctx context.Context) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if s.isClosed() {
		return newAPIError(CodeServerError, "realtime session already closed")
	}
	return nil
}

func (s *RealtimeSession) sendJSONEvent(ctx context.Context, event int32, body map[string]any) error {
	if body == nil {
		body = map[string]any{}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return wrapError(err, "marshal realtime event payload")
	}

	packet, err := protocol.BuildFullClientJSONWithEvent(event, s.sessionID, payload)
	if err != nil {
		return wrapError(err, "encode realtime event")
	}

	if err := s.conn.writeBinary(ctx, packet); err != nil {
		return wrapError(err, "send realtime event")
	}
	return nil
}

func (s *RealtimeSession) beginRecv() error {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()

	if s.recvBusy {
		return newAPIError(CodeParamError, "concurrent Recv is not supported")
	}
	s.recvBusy = true
	return nil
}

func (s *RealtimeSession) endRecv() {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()

	s.recvBusy = false
}

func (s *RealtimeSession) receiveLoop() {
	defer close(s.recvDone)
	defer close(s.eventCh)
	defer close(s.errCh)

	for {
		if s.isClosed() {
			return
		}

		msgType, payload, err := s.conn.conn.ReadMessage()
		if err != nil {
			if s.isClosed() {
				return
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			s.pushErr(wrapError(err, "realtime websocket read"))
			return
		}

		switch msgType {
		case websocket.BinaryMessage:
			frame, err := protocol.ParseServerFrame(payload)
			if err != nil {
				s.pushErr(wrapError(err, "parse realtime frame"))
				return
			}

			evt, fatalErr := s.decodeFrame(frame)
			if evt != nil {
				if err := s.enqueueEvent(evt); err != nil {
					s.pushErr(err)
					return
				}
			}
			if fatalErr != nil {
				s.pushErr(fatalErr)
				return
			}
		case websocket.TextMessage:
			s.pushErr(withErrorMetadata(parseWSErrorPayload(payload, 0), responseMetadata{ReqID: s.sessionID, ConnectID: s.conn.connectID}))
			return
		default:
			// Ignore unsupported message types.
		}
	}
}

func (s *RealtimeSession) decodeFrame(frame *protocol.ParsedFrame) (*RealtimeEvent, error) {
	evt := &RealtimeEvent{
		Payload: copyBytes(frame.Payload),
	}

	if frame.HasSequence {
		evt.Sequence = frame.Sequence
	}
	if frame.HasEvent {
		evt.Type = RealtimeEventType(frame.Event)
	}
	if frame.SessionID != "" {
		evt.SessionID = frame.SessionID
	}
	if frame.ConnectID != "" {
		evt.ConnectID = frame.ConnectID
	}

	if frame.MessageType == protocol.MessageTypeError {
		parsedErr := withErrorMetadata(parseWSErrorPayload(frame.Payload, frame.ErrorCode), responseMetadata{ReqID: s.sessionID, ConnectID: s.conn.connectID})
		if apiErr, ok := AsError(parsedErr); ok {
			evt.Error = apiErr
			evt.ReqID = apiErr.ReqID
			evt.TraceID = apiErr.TraceID
			evt.LogID = apiErr.LogID
		}
		if evt.Type == 0 {
			evt.Type = EventSessionFailed
		}
		return evt, wrapRealtimeEventError(evt.Type, parsedErr)
	}

	if frame.MessageType == protocol.MessageTypeAudioOnlyServer {
		evt.Audio = copyBytes(frame.Payload)
		if evt.Type == 0 {
			evt.Type = EventTTSAudioData
		}
	}

	decodeEventPayload(evt)
	if evt.Error != nil {
		withErrorMetadata(evt.Error, responseMetadata{ReqID: s.sessionID, ConnectID: s.conn.connectID})
		evt.ReqID = evt.Error.ReqID
		evt.TraceID = evt.Error.TraceID
		evt.LogID = evt.Error.LogID
	}
	s.markFinalOnce(evt)

	if evt.Error != nil && (evt.Type == EventSessionFailed || evt.Type == EventConnectionFailed) {
		return evt, wrapRealtimeEventError(evt.Type, evt.Error)
	}
	if evt.Type == EventSessionFailed || evt.Type == EventConnectionFailed {
		return evt, wrapRealtimeEventError(evt.Type, newAPIError(CodeServerError, "realtime session failed"))
	}

	return evt, nil
}

func (s *RealtimeSession) enqueueEvent(evt *RealtimeEvent) error {
	timeout := s.backpressureTimeout
	if timeout <= 0 {
		timeout = defaultRealtimeBackpressureTimeout
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case s.eventCh <- evt:
		return nil
	case <-s.closed:
		return nil
	case <-timer.C:
		return newAPIError(CodeServerError, "realtime event buffer full")
	}
}

func (s *RealtimeSession) pushErr(err error) {
	if err == nil {
		return
	}
	s.errOnce.Do(func() {
		select {
		case s.errCh <- err:
		case <-s.closed:
		}
	})
}

func (s *RealtimeSession) resetTurnFinalState() {
	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	s.turnFinalDelivered = false
	s.audioTurnInProgress = false
}

func (s *RealtimeSession) beginAudioTurn() {
	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	if !s.audioTurnInProgress || s.turnFinalDelivered {
		s.turnFinalDelivered = false
	}
	s.audioTurnInProgress = true
}

func (s *RealtimeSession) markFinalOnce(evt *RealtimeEvent) {
	if evt == nil {
		return
	}

	candidate := evt.IsFinal ||
		evt.Type == EventASREnded ||
		evt.Type == EventTTSFinished ||
		evt.Type == EventChatEnded ||
		evt.Type == EventSessionFinished
	if !candidate {
		return
	}

	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	if s.turnFinalDelivered {
		evt.IsFinal = false
		return
	}

	evt.IsFinal = true
	s.turnFinalDelivered = true
	s.audioTurnInProgress = false
}

func (s *RealtimeSession) appendHistory(message RealtimeConversationMessage) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.history = append(s.history, message)
}

func (s *RealtimeSession) snapshotSessionState() ([]RealtimeConversationMessage, RealtimePromptConfig, RealtimeGenerationProps) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()

	return cloneConversationHistory(s.history), clonePromptConfig(s.prompt), cloneGenerationProps(s.props)
}

func normalizeRealtimeConfig(cfg *RealtimeConfig) (RealtimeConfig, error) {
	base := DefaultRealtimeConfig()
	if cfg != nil {
		base = *cfg
	}

	if base.TTS.AudioConfig.Format == "" {
		base.TTS.AudioConfig.Format = FormatPCMS16LE
	}
	if base.TTS.AudioConfig.SampleRate == 0 {
		base.TTS.AudioConfig.SampleRate = SampleRate24000
	}
	if base.TTS.AudioConfig.Channel == 0 {
		base.TTS.AudioConfig.Channel = 1
	}
	if base.TTS.AudioConfig.Bits == 0 {
		base.TTS.AudioConfig.Bits = 16
	}
	if base.ASR.Language == "" {
		base.ASR.Language = LanguageZhCN
	}
	if base.EventBuffer <= 0 {
		base.EventBuffer = defaultRealtimeEventBuffer
	}
	if base.BackpressureTimeout <= 0 {
		base.BackpressureTimeout = defaultRealtimeBackpressureTimeout
	}

	if strings.TrimSpace(base.TTS.Speaker) == "" {
		return base, newAPIError(CodeParamError, "tts.speaker is required")
	}
	if strings.TrimSpace(base.ResourceID) != "" {
		return base, newAPIError(CodeParamError, "resource_id must be configured with WithResourceID")
	}
	if err := util.ValidateFormat(string(base.TTS.AudioConfig.Format)); err != nil {
		return base, newAPIError(CodeParamError, err.Error())
	}
	if err := util.ValidateSampleRate(int(base.TTS.AudioConfig.SampleRate)); err != nil {
		return base, newAPIError(CodeParamError, err.Error())
	}
	if err := util.ValidateChannel(base.TTS.AudioConfig.Channel); err != nil {
		return base, newAPIError(CodeParamError, err.Error())
	}
	if err := util.ValidateBits(base.TTS.AudioConfig.Bits); err != nil {
		return base, newAPIError(CodeParamError, err.Error())
	}
	if err := validateRealtimeInputMode(base.InputMode); err != nil {
		return base, err
	}
	model, err := normalizeRealtimeModel(base.Model)
	if err != nil {
		return base, err
	}
	if model == "" {
		return base, newAPIError(CodeParamError, "realtime model is required")
	}
	base.Model = model
	if err := normalizeRealtimeInstructions(&base); err != nil {
		return base, err
	}
	if err := validateRealtimeRate("speech_rate", base.TTS.AudioConfig.SpeechRate); err != nil {
		return base, err
	}
	if err := validateRealtimeRate("loudness_rate", base.TTS.AudioConfig.LoudnessRate); err != nil {
		return base, err
	}
	if base.ASR.AudioInfo != nil {
		if base.ASR.AudioInfo.Format != "" {
			if base.ASR.AudioInfo.Format != FormatSpeechOpus {
				if err := util.ValidateFormat(string(base.ASR.AudioInfo.Format)); err != nil {
					return base, newAPIError(CodeParamError, err.Error())
				}
			}
			if base.ASR.AudioInfo.Format == FormatSpeechOpus && base.ASR.AudioInfo.SampleRate != 0 && base.ASR.AudioInfo.SampleRate != SampleRate16000 {
				return base, newAPIError(CodeParamError, "speech_opus sample_rate must be 16000")
			}
		}
		if base.ASR.AudioInfo.SampleRate != 0 {
			if err := util.ValidateSampleRate(int(base.ASR.AudioInfo.SampleRate)); err != nil {
				return base, newAPIError(CodeParamError, err.Error())
			}
		}
		if base.ASR.AudioInfo.Channel != 0 {
			if err := util.ValidateChannel(base.ASR.AudioInfo.Channel); err != nil {
				return base, newAPIError(CodeParamError, err.Error())
			}
		}
	}
	if base.ASR.Extra != nil && base.ASR.Extra.EndSmoothWindowMS != 0 {
		if base.ASR.Extra.EndSmoothWindowMS < 500 || base.ASR.Extra.EndSmoothWindowMS > 50000 {
			return base, newAPIError(CodeParamError, "asr.extra.end_smooth_window_ms must be between 500 and 50000")
		}
	}
	if err := validateRealtimeCapabilities(base); err != nil {
		return base, err
	}

	base.History = cloneConversationHistory(base.History)
	base.Prompt = clonePromptConfig(base.Prompt)
	base.Props = cloneGenerationProps(base.Props)

	return base, nil
}

func normalizeRealtimeInstructions(cfg *RealtimeConfig) error {
	if cfg == nil {
		return newAPIError(CodeParamError, "realtime config is required")
	}

	switch cfg.Model {
	case RealtimeModelO20:
		if cfg.Dialog.CharacterManifest != "" {
			return newAPIError(CodeParamError, "dialog.character_manifest is not supported by O20")
		}
		if cfg.Instructions != "" {
			if cfg.Dialog.SystemRole != "" && cfg.Dialog.SystemRole != cfg.Instructions {
				return newAPIError(CodeParamError, "instructions conflict with dialog.system_role")
			}
			cfg.Dialog.SystemRole = cfg.Instructions
		}
	case RealtimeModelSC20:
		if cfg.Dialog.BotName != "" || cfg.Dialog.SystemRole != "" || cfg.Dialog.SpeakingStyle != "" {
			return newAPIError(CodeParamError, "dialog O20 fields are not supported by SC20")
		}
		if cfg.Instructions != "" {
			if cfg.Dialog.CharacterManifest != "" && cfg.Dialog.CharacterManifest != cfg.Instructions {
				return newAPIError(CodeParamError, "instructions conflict with dialog.character_manifest")
			}
			cfg.Dialog.CharacterManifest = cfg.Instructions
		}
	}
	return nil
}

func validateRealtimeCapabilities(cfg RealtimeConfig) error {
	if cfg.Dialog.Extra != nil {
		extra := cfg.Dialog.Extra
		if extra.VolcWebsearchResultCount < 0 || extra.VolcWebsearchResultCount > 10 {
			return newAPIError(CodeParamError, "dialog.extra.volc_websearch_result_count must be in range [0,10]")
		}
		switch extra.VolcWebsearchType {
		case "", "web", "web_summary", "web_agent":
		default:
			return newAPIError(CodeParamError, "dialog.extra.volc_websearch_type must be one of [web,web_summary,web_agent]")
		}
		if boolValue(extra.EnableVolcWebsearch) {
			if strings.TrimSpace(extra.VolcWebsearchAPIKey) == "" {
				return newAPIError(CodeParamError, "dialog.extra.volc_websearch_api_key is required when web search is enabled")
			}
			if extra.VolcWebsearchType == "web_agent" && strings.TrimSpace(extra.VolcWebsearchBotID) == "" {
				return newAPIError(CodeParamError, "dialog.extra.volc_websearch_bot_id is required for web_agent")
			}
		}
		if cfg.Model == RealtimeModelSC20 && boolValue(extra.EnableMusic) {
			return newAPIError(CodeParamError, "dialog.extra.enable_music is not supported by SC20")
		}
	}

	if cfg.TTS.Extra != nil {
		switch cfg.TTS.Extra.ExplicitDialect {
		case "", "dongbei", "sichuan", "shaanxi":
		default:
			return newAPIError(CodeParamError, "tts.extra.explicit_dialect must be one of [dongbei,sichuan,shaanxi]")
		}
		if cfg.Model == RealtimeModelSC20 && cfg.TTS.Extra.TTS20Model != "" {
			return newAPIError(CodeParamError, "tts.extra.tts_2.0_model is not supported by SC20")
		}
	}

	return nil
}

func buildRealtimeStartPayload(cfg RealtimeConfig) ([]byte, error) {
	payload := map[string]any{
		"asr": buildRealtimeASRPayload(cfg.ASR),
		"tts": map[string]any{
			"speaker": cfg.TTS.Speaker,
			"audio_config": map[string]any{
				"channel":     cfg.TTS.AudioConfig.Channel,
				"format":      cfg.TTS.AudioConfig.Format,
				"sample_rate": cfg.TTS.AudioConfig.SampleRate,
				"bits":        cfg.TTS.AudioConfig.Bits,
			},
		},
		"dialog": map[string]any{},
	}

	tts := payload["tts"].(map[string]any)
	ttsAudio := tts["audio_config"].(map[string]any)
	if cfg.TTS.AudioConfig.SpeechRate != 0 {
		ttsAudio["speech_rate"] = cfg.TTS.AudioConfig.SpeechRate
	}
	if cfg.TTS.AudioConfig.LoudnessRate != 0 {
		ttsAudio["loudness_rate"] = cfg.TTS.AudioConfig.LoudnessRate
	}
	if cfg.TTS.Extra != nil {
		if extra := structToMap(cfg.TTS.Extra); len(extra) > 0 {
			tts["extra"] = extra
		}
	}

	dialog := payload["dialog"].(map[string]any)
	if cfg.Dialog.DialogID != "" {
		dialog["dialog_id"] = cfg.Dialog.DialogID
	}
	if cfg.Dialog.BotName != "" {
		dialog["bot_name"] = cfg.Dialog.BotName
	}
	if cfg.Dialog.SystemRole != "" {
		dialog["system_role"] = cfg.Dialog.SystemRole
	}
	if cfg.Dialog.SpeakingStyle != "" {
		dialog["speaking_style"] = cfg.Dialog.SpeakingStyle
	}
	if cfg.Dialog.CharacterManifest != "" {
		dialog["character_manifest"] = cfg.Dialog.CharacterManifest
	}
	if cfg.Dialog.Location != nil {
		if location := structToMap(cfg.Dialog.Location); len(location) > 0 {
			dialog["location"] = location
		}
	}
	if len(cfg.Dialog.DialogContext) > 0 {
		dialog["dialog_context"] = cfg.Dialog.DialogContext
	}
	dialogExtra := structToMap(cfg.Dialog.Extra)
	if cfg.InputMode != RealtimeInputModeDefault {
		dialogExtra["input_mod"] = string(cfg.InputMode)
	}
	if cfg.Model != "" {
		dialogExtra["model"] = string(cfg.Model)
	}
	if len(dialogExtra) > 0 {
		dialog["extra"] = dialogExtra
	}

	if cfg.Prompt.System != "" || len(cfg.Prompt.Variables) > 0 {
		payload["prompt"] = cfg.Prompt
	}
	if hasRealtimeProps(cfg.Props) {
		payload["props"] = cfg.Props
	}
	if len(cfg.History) > 0 {
		payload["history"] = cfg.History
	}

	return json.Marshal(payload)
}

func buildRealtimeASRPayload(cfg RealtimeASRConfig) map[string]any {
	payload := map[string]any{}
	if cfg.Language != "" {
		payload["language"] = cfg.Language
	}
	if cfg.AudioInfo != nil {
		if audioInfo := structToMap(cfg.AudioInfo); len(audioInfo) > 0 {
			payload["audio_info"] = audioInfo
		}
	}
	if cfg.Extra != nil {
		if extra := structToMap(cfg.Extra); len(extra) > 0 {
			payload["extra"] = extra
		}
	}
	return payload
}

func buildRealtimeUpdateConfigPayload(cfg RealtimeUpdateConfig, model RealtimeModelVersion) (map[string]any, error) {
	payload := map[string]any{}
	if cfg.TTS != nil {
		if strings.TrimSpace(cfg.TTS.Speaker) == "" {
			return nil, newAPIError(CodeParamError, "tts.speaker is required")
		}
		if cfg.TTS.AudioConfig.Channel != 0 || cfg.TTS.AudioConfig.Format != "" || cfg.TTS.AudioConfig.SampleRate != 0 || cfg.TTS.AudioConfig.Bits != 0 || cfg.TTS.Extra != nil {
			return nil, newAPIError(CodeParamError, "UpdateConfig supports only tts.speaker, speech_rate, and loudness_rate")
		}
		if err := validateRealtimeRate("speech_rate", cfg.TTS.AudioConfig.SpeechRate); err != nil {
			return nil, err
		}
		if err := validateRealtimeRate("loudness_rate", cfg.TTS.AudioConfig.LoudnessRate); err != nil {
			return nil, err
		}
		audioConfig := map[string]any{}
		if cfg.TTS.AudioConfig.SpeechRate != 0 {
			audioConfig["speech_rate"] = cfg.TTS.AudioConfig.SpeechRate
		}
		if cfg.TTS.AudioConfig.LoudnessRate != 0 {
			audioConfig["loudness_rate"] = cfg.TTS.AudioConfig.LoudnessRate
		}
		tts := map[string]any{"speaker": strings.TrimSpace(cfg.TTS.Speaker)}
		if len(audioConfig) > 0 {
			tts["audio_config"] = audioConfig
		}
		payload["tts"] = tts
	}
	if cfg.Dialog != nil {
		if cfg.Dialog.CharacterManifest != "" || len(cfg.Dialog.DialogContext) > 0 || cfg.Dialog.Extra != nil {
			return nil, newAPIError(CodeParamError, "UpdateConfig contains unsupported dialog fields")
		}
		if model == RealtimeModelSC20 && (cfg.Dialog.BotName != "" || cfg.Dialog.SystemRole != "" || cfg.Dialog.SpeakingStyle != "") {
			return nil, newAPIError(CodeParamError, "UpdateConfig O20 dialog fields are not supported by SC20")
		}
		dialog := map[string]any{}
		if cfg.Dialog.DialogID != "" {
			dialog["dialog_id"] = cfg.Dialog.DialogID
		}
		if cfg.Dialog.BotName != "" {
			dialog["bot_name"] = cfg.Dialog.BotName
		}
		if cfg.Dialog.SystemRole != "" {
			dialog["system_role"] = cfg.Dialog.SystemRole
		}
		if cfg.Dialog.SpeakingStyle != "" {
			dialog["speaking_style"] = cfg.Dialog.SpeakingStyle
		}
		if cfg.Dialog.Location != nil {
			if location := structToMap(cfg.Dialog.Location); len(location) > 0 {
				dialog["location"] = location
			}
		}
		if len(dialog) > 0 {
			payload["dialog"] = dialog
		}
	}
	if len(payload) == 0 {
		return nil, newAPIError(CodeParamError, "update config is empty")
	}
	return payload, nil
}

func buildRealtimeConversationCreateItems(items []RealtimeConversationItem) ([]map[string]any, error) {
	if len(items) == 0 || len(items) > 40 {
		return nil, newAPIError(CodeParamError, "conversation create requires 1 to 40 items")
	}
	if len(items)%2 != 0 {
		return nil, newAPIError(CodeParamError, "conversation create requires complete user/assistant pairs")
	}

	withTimestamp := items[0].Timestamp != 0
	previousTimestamp := int64(0)
	result := make([]map[string]any, 0, len(items))
	for i, item := range items {
		wantRole := "user"
		if i%2 == 1 {
			wantRole = "assistant"
		}
		if item.Role != wantRole {
			return nil, newAPIError(CodeParamError, "conversation create roles must alternate user and assistant")
		}
		if strings.TrimSpace(item.Text) == "" {
			return nil, newAPIError(CodeParamError, "conversation create text is required")
		}
		if strings.TrimSpace(item.ItemID) != "" {
			return nil, newAPIError(CodeParamError, "conversation create does not accept item_id")
		}
		if (item.Timestamp != 0) != withTimestamp {
			return nil, newAPIError(CodeParamError, "conversation create timestamps must be all set or all omitted")
		}
		if withTimestamp && item.Timestamp <= previousTimestamp {
			return nil, newAPIError(CodeParamError, "conversation create timestamps must be strictly increasing")
		}
		if withTimestamp && realtimeTimestampIsFuture(item.Timestamp, time.Now()) {
			return nil, newAPIError(CodeParamError, "conversation create timestamp must not be in the future")
		}

		projected := map[string]any{"role": item.Role, "text": item.Text}
		if withTimestamp {
			projected["timestamp"] = item.Timestamp
			previousTimestamp = item.Timestamp
		}
		result = append(result, projected)
	}
	return result, nil
}

func realtimeTimestampIsFuture(timestamp int64, now time.Time) bool {
	switch {
	case timestamp >= 100_000_000_000_000_000:
		return timestamp > now.UnixNano()
	case timestamp >= 100_000_000_000_000:
		return timestamp > now.UnixMicro()
	case timestamp >= 100_000_000_000:
		return timestamp > now.UnixMilli()
	default:
		return timestamp > now.Unix()
	}
}

func buildRealtimeConversationUpdateItems(items []RealtimeConversationItem) ([]map[string]any, error) {
	if len(items) == 0 || len(items) > 40 {
		return nil, newAPIError(CodeParamError, "conversation update requires 1 to 40 items")
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ItemID) == "" || strings.TrimSpace(item.Text) == "" {
			return nil, newAPIError(CodeParamError, "conversation update requires item_id and text")
		}
		if item.Role != "" || item.Timestamp != 0 {
			return nil, newAPIError(CodeParamError, "conversation update accepts only item_id and text")
		}
		result = append(result, map[string]any{"item_id": item.ItemID, "text": item.Text})
	}
	return result, nil
}

func buildRealtimeTTSPayload(cfg RealtimeTTSConfig) map[string]any {
	tts := map[string]any{}
	if strings.TrimSpace(cfg.Speaker) != "" {
		tts["speaker"] = strings.TrimSpace(cfg.Speaker)
	}
	audioConfig := map[string]any{}
	if cfg.AudioConfig.Channel != 0 {
		audioConfig["channel"] = cfg.AudioConfig.Channel
	}
	if cfg.AudioConfig.Format != "" {
		audioConfig["format"] = cfg.AudioConfig.Format
	}
	if cfg.AudioConfig.SampleRate != 0 {
		audioConfig["sample_rate"] = cfg.AudioConfig.SampleRate
	}
	if cfg.AudioConfig.Bits != 0 {
		audioConfig["bits"] = cfg.AudioConfig.Bits
	}
	if cfg.AudioConfig.SpeechRate != 0 {
		audioConfig["speech_rate"] = cfg.AudioConfig.SpeechRate
	}
	if cfg.AudioConfig.LoudnessRate != 0 {
		audioConfig["loudness_rate"] = cfg.AudioConfig.LoudnessRate
	}
	if len(audioConfig) > 0 {
		tts["audio_config"] = audioConfig
	}
	if cfg.Extra != nil {
		if extra := structToMap(cfg.Extra); len(extra) > 0 {
			tts["extra"] = extra
		}
	}
	return tts
}

func buildRealtimeDialogPayload(cfg RealtimeDialogConfig) map[string]any {
	dialog := map[string]any{}
	if cfg.DialogID != "" {
		dialog["dialog_id"] = cfg.DialogID
	}
	if cfg.BotName != "" {
		dialog["bot_name"] = cfg.BotName
	}
	if cfg.SystemRole != "" {
		dialog["system_role"] = cfg.SystemRole
	}
	if cfg.SpeakingStyle != "" {
		dialog["speaking_style"] = cfg.SpeakingStyle
	}
	if cfg.CharacterManifest != "" {
		dialog["character_manifest"] = cfg.CharacterManifest
	}
	if cfg.Location != nil {
		if location := structToMap(cfg.Location); len(location) > 0 {
			dialog["location"] = location
		}
	}
	if len(cfg.DialogContext) > 0 {
		dialog["dialog_context"] = cfg.DialogContext
	}
	if cfg.Extra != nil {
		if extra := structToMap(cfg.Extra); len(extra) > 0 {
			dialog["extra"] = extra
		}
	}
	return dialog
}

func decodeEventPayload(evt *RealtimeEvent) {
	if evt == nil || len(evt.Payload) == 0 {
		return
	}

	var payload struct {
		SessionID string `json:"session_id"`
		DialogID  string `json:"dialog_id"`

		Text    string `json:"text"`
		Content string `json:"content"`
		Audio   string `json:"audio"`

		QuestionID string `json:"question_id"`
		ReplyID    string `json:"reply_id"`
		TTSType    string `json:"tts_type"`
		StatusCode any    `json:"status_code"`

		IsFinal bool `json:"is_final"`

		Code    any    `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error"`

		Usage   *RealtimeUsage `json:"usage,omitempty"`
		Results []struct {
			Text      string `json:"text"`
			IsInterim bool   `json:"is_interim"`
		} `json:"results,omitempty"`
		Items []RealtimeConversationItem `json:"items,omitempty"`

		ASRInfo *struct {
			Text    string `json:"text"`
			IsFinal bool   `json:"is_final"`
		} `json:"asr_info,omitempty"`
		TTSInfo *struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		} `json:"tts_info,omitempty"`
	}

	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return
	}

	if payload.SessionID != "" {
		evt.SessionID = payload.SessionID
	}
	if payload.DialogID != "" {
		evt.DialogID = payload.DialogID
	}
	if payload.Message != "" {
		evt.Message = payload.Message
	}
	meta := parseResponseMetadata(evt.Payload, responseMetadata{ReqID: evt.ReqID, TraceID: evt.TraceID, LogID: evt.LogID})
	evt.ReqID = meta.ReqID
	evt.TraceID = meta.TraceID
	evt.LogID = meta.LogID

	if payload.Content != "" {
		evt.Text = payload.Content
	} else if payload.Text != "" {
		evt.Text = payload.Text
	}
	evt.QuestionID = payload.QuestionID
	evt.ReplyID = payload.ReplyID
	evt.TTSType = payload.TTSType
	evt.StatusCode = realtimeString(payload.StatusCode)
	evt.Usage = payload.Usage
	if len(payload.Results) > 0 {
		evt.Results = make([]RealtimeASRResult, 0, len(payload.Results))
		for _, result := range payload.Results {
			evt.Results = append(evt.Results, RealtimeASRResult{
				Text:      result.Text,
				IsInterim: result.IsInterim,
			})
			if evt.Text == "" && result.Text != "" {
				evt.Text = result.Text
			}
			evt.IsFinal = evt.IsFinal || !result.IsInterim
		}
	}
	if len(payload.Items) > 0 {
		evt.Items = payload.Items
	}

	if payload.ASRInfo != nil {
		if payload.ASRInfo.Text != "" {
			evt.Text = payload.ASRInfo.Text
		}
		evt.IsFinal = evt.IsFinal || payload.ASRInfo.IsFinal
	}
	if payload.TTSInfo != nil {
		if payload.TTSInfo.Content != "" {
			evt.Text = payload.TTSInfo.Content
		} else if payload.TTSInfo.Text != "" {
			evt.Text = payload.TTSInfo.Text
		}
	}

	evt.IsFinal = evt.IsFinal || payload.IsFinal

	if evt.Audio == nil && payload.Audio != "" {
		decoded, err := base64.StdEncoding.DecodeString(payload.Audio)
		if err == nil {
			evt.Audio = decoded
		}
	}

	code := realtimeStatusCode(payload.Code)
	statusCode := realtimeStatusCode(payload.StatusCode)
	shouldMapError := code != 0
	if (evt.Type == EventConnectionFailed || evt.Type == EventSessionFailed) && (payload.Error != "" || payload.Message != "") {
		shouldMapError = true
	}
	if evt.Type == EventDialogCommonError && (evt.StatusCode != "" || payload.Message != "") {
		shouldMapError = true
	}
	if evt.Type == EventConversationDeleted && evt.StatusCode != "" && !realtimeSuccessfulStatus(evt.StatusCode) {
		shouldMapError = true
	}
	if shouldMapError {
		message := payload.Message
		if message == "" {
			message = payload.Error
		}
		if message == "" {
			message = "realtime event error"
		}
		if code == 0 {
			code = statusCode
		}
		if code == 0 {
			code = CodeServerError
		}
		evt.Error = &Error{
			Code:    code,
			Message: message,
			ReqID:   meta.ReqID,
			TraceID: meta.TraceID,
			LogID:   meta.LogID,
		}
	}
}

func realtimePayloadError(data []byte, fallbackCode int, fallbackMessage string) error {
	evt := &RealtimeEvent{Payload: copyBytes(data)}
	decodeEventPayload(evt)
	if evt.Error != nil {
		return evt.Error
	}

	var payload struct {
		Code       any    `json:"code"`
		StatusCode any    `json:"status_code"`
		Message    string `json:"message"`
		Error      string `json:"error"`
	}
	_ = json.Unmarshal(data, &payload)
	code := realtimeStatusCode(payload.Code)
	if code == 0 {
		code = realtimeStatusCode(payload.StatusCode)
	}
	if code == 0 {
		code = fallbackCode
	}
	if code == 0 {
		code = CodeServerError
	}
	message := payload.Message
	if message == "" {
		message = payload.Error
	}
	if message == "" {
		message = fallbackMessage
	}
	meta := parseResponseMetadata(data, responseMetadata{})
	return &Error{Code: code, Message: message, ReqID: meta.ReqID, TraceID: meta.TraceID, LogID: meta.LogID, ConnectID: meta.ConnectID}
}

func realtimeStatusCode(value any) int {
	text := strings.TrimSpace(realtimeString(value))
	if text == "" {
		return 0
	}
	code, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return code
}

func realtimeSuccessfulStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "", "0", "200", "20000000", strconv.Itoa(CodeSuccess):
		return true
	default:
		return false
	}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func validateRealtimeInputMode(mode RealtimeInputMode) error {
	switch mode {
	case RealtimeInputModeDefault, RealtimeInputModeKeepAlive, RealtimeInputModePushToTalk, RealtimeInputModeText, RealtimeInputModeAudioFile:
		return nil
	default:
		return newAPIError(CodeParamError, "unsupported realtime input mode: "+string(mode))
	}
}

func normalizeRealtimeModel(model RealtimeModelVersion) (RealtimeModelVersion, error) {
	value := strings.TrimSpace(strings.ToLower(string(model)))
	switch value {
	case "":
		return "", nil
	case "1.2.1.1", "o", "omni", "o2", "o2.0", "o20":
		return RealtimeModelO20, nil
	case "2.2.0.0", "sc", "sc2", "sc2.0", "sc20":
		return RealtimeModelSC20, nil
	default:
		return "", newAPIError(CodeParamError, "unsupported realtime model: "+string(model))
	}
}

func validateRealtimeRate(name string, value int) error {
	if value < -50 || value > 100 {
		return newAPIError(CodeParamError, name+" must be in range [-50,100]")
	}
	return nil
}

func realtimeString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func hasRealtimeProps(props RealtimeGenerationProps) bool {
	return props.Temperature != 0 ||
		props.TopP != 0 ||
		props.MaxTokens != 0 ||
		props.PresencePenalty != 0 ||
		props.FrequencyPenalty != 0
}

func structToMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	if out == nil {
		return map[string]any{}
	}
	for key, value := range out {
		if value == nil {
			delete(out, key)
		}
	}
	return out
}

func cloneConversationHistory(history []RealtimeConversationMessage) []RealtimeConversationMessage {
	if len(history) == 0 {
		return nil
	}
	out := make([]RealtimeConversationMessage, len(history))
	copy(out, history)
	return out
}

func clonePromptConfig(prompt RealtimePromptConfig) RealtimePromptConfig {
	out := prompt
	if len(prompt.Variables) > 0 {
		out.Variables = make(map[string]string, len(prompt.Variables))
		maps.Copy(out.Variables, prompt.Variables)
	}
	return out
}

func cloneGenerationProps(props RealtimeGenerationProps) RealtimeGenerationProps {
	return props
}

func readWSMessageWithContext(ctx context.Context, conn transport.WSConn) (int, []byte, error) {
	if ctx == nil {
		return conn.ReadMessage()
	}

	type readResult struct {
		msgType int
		payload []byte
		err     error
	}

	resultCh := make(chan readResult, 1)
	go func() {
		msgType, payload, err := conn.ReadMessage()
		resultCh <- readResult{msgType: msgType, payload: payload, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = conn.Close()
		return 0, nil, ctx.Err()
	case result := <-resultCh:
		return result.msgType, result.payload, result.err
	}
}

func wrapRealtimeEventError(eventType RealtimeEventType, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("realtime event %d: %w", eventType, err)
}

func copyBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
