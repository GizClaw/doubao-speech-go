package doubaospeech

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/astproto"
	"github.com/GizClaw/doubao-speech-go/internal/auth"
	"github.com/GizClaw/doubao-speech-go/internal/transport"
	"github.com/GizClaw/doubao-speech-go/internal/util"
	"github.com/gorilla/websocket"
)

const (
	astTranslateEndpointPath = "/api/v4/ast/v2/translate"

	defaultASTTranslateEventBuffer         = 64
	defaultASTTranslateBackpressureTimeout = 2 * time.Second
	defaultASTTranslateCloseWaitTimeout    = 2 * time.Second
)

type ASTTranslateService struct {
	client *Client
	dialer transport.WSDialer
}

func newASTTranslateService(c *Client) *ASTTranslateService {
	return &ASTTranslateService{
		client: c,
		dialer: transport.NewGorillaDialer(nil),
	}
}

func (s *ASTTranslateService) OpenSession(ctx context.Context, cfg *ASTTranslateConfig) (*ASTTranslateSession, error) {
	normalized, err := normalizeASTTranslateConfig(cfg)
	if err != nil {
		return nil, err
	}

	connectID := util.NewReqID("ast")
	resourceID := s.client.resolveResourceID(normalized.ResourceID, ResourceASTTranslate)
	headers := buildASTTranslateHeaders(s.client.authCredentials(), resourceID, connectID)

	endpoint := strings.TrimRight(s.client.config.wsURL, "/") + astTranslateEndpointPath
	conn, resp, err := s.dialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		return nil, wsConnectError(err, resp, responseMetadata{ReqID: normalized.SessionID, ConnectID: connectID})
	}

	session := &ASTTranslateSession{
		conn:                conn,
		client:              s.client,
		cfg:                 normalized,
		sessionID:           normalized.SessionID,
		connectID:           connectID,
		eventCh:             make(chan *ASTTranslateEvent, resolvedASTTranslateEventBuffer(normalized)),
		errCh:               make(chan error, 1),
		closed:              make(chan struct{}),
		recvDone:            make(chan struct{}),
		backpressureTimeout: resolvedASTTranslateBackpressureTimeout(normalized),
	}

	if err := session.sendStartSession(ctx); err != nil {
		_ = session.abortOpen()
		return nil, wrapError(err, "send ast start session")
	}

	evt, err := session.readEventWithContext(ctx)
	if err != nil {
		_ = session.abortOpen()
		return nil, wrapError(err, "read ast start response")
	}
	if evt.Type != ASTEventSessionStarted {
		_ = session.abortOpen()
		return nil, fmt.Errorf("unexpected ast start response event: %d", evt.Type)
	}
	if evt.Error != nil {
		_ = session.abortOpen()
		return nil, evt.Error
	}

	go session.receiveLoop()
	return session, nil
}

type ASTTranslateSession struct {
	conn      transport.WSConn
	client    *Client
	cfg       ASTTranslateConfig
	sessionID string
	connectID string

	eventCh  chan *ASTTranslateEvent
	errCh    chan error
	closed   chan struct{}
	recvDone chan struct{}

	backpressureTimeout time.Duration

	writeMu  sync.Mutex
	recvMu   sync.Mutex
	recvBusy bool

	errOnce   sync.Once
	closeOnce sync.Once
	closeErr  error
}

func (s *ASTTranslateSession) SessionID() string {
	return s.sessionID
}

func (s *ASTTranslateSession) SendAudio(ctx context.Context, audio []byte) error {
	if len(audio) == 0 {
		return newAPIError(CodeParamError, "audio payload is empty")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	req := &astproto.TranslateRequest{
		RequestMeta: &astproto.RequestMeta{SessionID: s.sessionID},
		Event:       astproto.EventTaskRequest,
		SourceAudio: &astproto.Audio{BinaryData: audio},
	}
	return s.writeRequest(ctx, req)
}

func (s *ASTTranslateSession) UpdateConfig(ctx context.Context, update ASTTranslateUpdate) error {
	if update.Corpus == nil {
		return newAPIError(CodeParamError, "ast update corpus is nil")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	req := &astproto.TranslateRequest{
		RequestMeta: &astproto.RequestMeta{SessionID: s.sessionID},
		Event:       astproto.EventUpdateConfig,
		Request: &astproto.ReqParams{
			Mode:   string(s.cfg.Mode),
			Corpus: astCorpus(update.Corpus),
		},
	}
	return s.writeRequest(ctx, req)
}

func (s *ASTTranslateSession) Finish(ctx context.Context) error {
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	req := &astproto.TranslateRequest{
		RequestMeta: &astproto.RequestMeta{SessionID: s.sessionID},
		Event:       astproto.EventFinishSession,
	}
	return s.writeRequest(ctx, req)
}

func (s *ASTTranslateSession) Recv() iter.Seq2[*ASTTranslateEvent, error] {
	return func(yield func(*ASTTranslateEvent, error) bool) {
		if err := s.beginRecv(); err != nil {
			yield(nil, err)
			return
		}
		defer s.endRecv()

		events := s.eventCh
		errs := s.errCh
		for events != nil || errs != nil {
			if evt, ok := receiveASTEventNow(events); ok {
				if !yield(evt, nil) {
					return
				}
				continue
			}
			if err, ok := receiveASTErrorNow(errs); ok {
				yield(nil, err)
				return
			}

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

func (s *ASTTranslateSession) RecvEvent(ctx context.Context) (*ASTTranslateEvent, error) {
	if err := s.beginRecv(); err != nil {
		return nil, err
	}
	defer s.endRecv()

	events := s.eventCh
	errs := s.errCh
	for events != nil || errs != nil {
		if evt, ok := receiveASTEventNow(events); ok {
			return evt, nil
		}
		if err, ok := receiveASTErrorNow(errs); ok {
			return nil, err
		}

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

func receiveASTEventNow(events <-chan *ASTTranslateEvent) (*ASTTranslateEvent, bool) {
	if events == nil {
		return nil, false
	}
	select {
	case evt, ok := <-events:
		if !ok {
			return nil, false
		}
		return evt, true
	default:
		return nil, false
	}
}

func receiveASTErrorNow(errs <-chan error) (error, bool) {
	if errs == nil {
		return nil, false
	}
	select {
	case err, ok := <-errs:
		if !ok {
			return nil, false
		}
		return err, true
	default:
		return nil, false
	}
}

func (s *ASTTranslateSession) Close() error {
	s.closeOnce.Do(func() {
		_ = s.Finish(context.Background())
		close(s.closed)
		s.closeErr = s.conn.Close()
		select {
		case <-s.recvDone:
		case <-time.After(defaultASTTranslateCloseWaitTimeout):
		}
	})
	return s.closeErr
}

func (s *ASTTranslateSession) abortOpen() error {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.closeErr = s.conn.Close()
		close(s.recvDone)
	})
	return s.closeErr
}

func (s *ASTTranslateSession) sendStartSession(ctx context.Context) error {
	denoise := false
	if s.cfg.Denoise != nil {
		denoise = *s.cfg.Denoise
	}
	req := &astproto.TranslateRequest{
		RequestMeta: &astproto.RequestMeta{SessionID: s.sessionID},
		Event:       astproto.EventStartSession,
		User:        astUser(s.client, s.cfg.User),
		SourceAudio: astSourceAudio(s.cfg.SourceAudio),
		Request: &astproto.ReqParams{
			Mode:           string(s.cfg.Mode),
			SourceLanguage: s.cfg.SourceLanguage,
			TargetLanguage: s.cfg.TargetLanguage,
			SpeakerID:      s.cfg.SpeakerID,
			TTSResourceID:  s.cfg.TTSResourceID,
			SpeechRate:     int32(s.cfg.SpeechRate),
			Corpus:         astCorpus(s.cfg.Corpus),
		},
		Denoise: &denoise,
	}
	if s.cfg.IsCustomSpeaker {
		req.Request.IsCustomSpeaker = &s.cfg.IsCustomSpeaker
	}
	if s.cfg.EnableSourceLanguageDetect {
		req.Request.EnableSourceLanguageDetect = &s.cfg.EnableSourceLanguageDetect
	}
	if shouldSendASTTargetAudio(s.cfg) {
		req.TargetAudio = astTargetAudio(s.cfg.TargetAudio)
	}
	return s.writeRequest(ctx, req)
}

func (s *ASTTranslateSession) writeRequest(ctx context.Context, req *astproto.TranslateRequest) error {
	payload, err := astproto.MarshalRequest(req)
	if err != nil {
		return wrapError(err, "marshal ast protobuf request")
	}
	if err := s.guardSend(ctx); err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.isClosed() {
		return newAPIError(CodeServerError, "ast translate session already closed")
	}
	if err := s.conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		return wrapError(err, "send ast protobuf request")
	}
	return nil
}

func (s *ASTTranslateSession) readEventWithContext(ctx context.Context) (*ASTTranslateEvent, error) {
	msgType, payload, err := readWSMessageWithContext(ctx, s.conn)
	if err != nil {
		return nil, err
	}
	return s.decodeMessage(msgType, payload)
}

func (s *ASTTranslateSession) decodeMessage(msgType int, payload []byte) (*ASTTranslateEvent, error) {
	switch msgType {
	case websocket.BinaryMessage:
		resp, err := astproto.UnmarshalResponse(payload)
		if err != nil {
			return nil, wrapError(err, "parse ast protobuf response")
		}
		evt := astEventFromProto(resp)
		evt.Payload = copyBytes(payload)
		if evt.SessionID == "" {
			evt.SessionID = s.sessionID
		}
		evt.ReqID = s.sessionID
		if evt.Error != nil {
			applyErrorMetadata(evt.Error, responseMetadata{ReqID: s.sessionID, ConnectID: s.connectID})
			return evt, wrapASTTranslateEventError(evt.Type, evt.Error)
		}
		if evt.Type == ASTEventSessionFailed || evt.Type == ASTEventSessionCanceled {
			err := newAPIError(CodeServerError, "ast translate session failed")
			applyErrorMetadata(err, responseMetadata{ReqID: s.sessionID, ConnectID: s.connectID})
			evt.Error = err
			return evt, wrapASTTranslateEventError(evt.Type, err)
		}
		return evt, nil
	case websocket.TextMessage:
		err := withErrorMetadata(parseWSErrorPayload(payload, 0), responseMetadata{ReqID: s.sessionID, ConnectID: s.connectID})
		return nil, err
	default:
		return nil, fmt.Errorf("unsupported ast websocket message type: %d", msgType)
	}
}

func (s *ASTTranslateSession) receiveLoop() {
	defer close(s.recvDone)
	defer close(s.eventCh)
	defer close(s.errCh)

	for {
		if s.isClosed() {
			return
		}

		msgType, payload, err := s.conn.ReadMessage()
		if err != nil {
			if s.isClosed() || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			s.pushErr(wrapError(err, "ast websocket read"))
			return
		}

		evt, fatalErr := s.decodeMessage(msgType, payload)
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
	}
}

func (s *ASTTranslateSession) beginRecv() error {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()

	if s.recvBusy {
		return newAPIError(CodeParamError, "concurrent Recv is not supported")
	}
	s.recvBusy = true
	return nil
}

func (s *ASTTranslateSession) endRecv() {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()
	s.recvBusy = false
}

func (s *ASTTranslateSession) enqueueEvent(evt *ASTTranslateEvent) error {
	timeout := s.backpressureTimeout
	if timeout <= 0 {
		timeout = defaultASTTranslateBackpressureTimeout
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case s.eventCh <- evt:
		return nil
	case <-s.closed:
		return nil
	case <-timer.C:
		return newAPIError(CodeServerError, "ast translate event buffer full")
	}
}

func (s *ASTTranslateSession) pushErr(err error) {
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

func (s *ASTTranslateSession) guardSend(ctx context.Context) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if s.isClosed() {
		return newAPIError(CodeServerError, "ast translate session already closed")
	}
	return nil
}

func (s *ASTTranslateSession) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

func normalizeASTTranslateConfig(cfg *ASTTranslateConfig) (ASTTranslateConfig, error) {
	normalized := DefaultASTTranslateConfig()
	if cfg != nil {
		normalized = *cfg
		if cfg.Denoise != nil {
			denoise := *cfg.Denoise
			normalized.Denoise = &denoise
		}
	}
	if normalized.SessionID == "" {
		normalized.SessionID = util.NewReqID("ast-session")
	}
	if normalized.Mode == "" {
		normalized.Mode = ASTTranslateModeS2T
	}
	if normalized.Mode != ASTTranslateModeS2T && normalized.Mode != ASTTranslateModeS2S {
		return normalized, newAPIError(CodeParamError, "ast translate mode must be s2t or s2s")
	}
	if strings.TrimSpace(normalized.SourceLanguage) == "" {
		return normalized, newAPIError(CodeParamError, "ast source language is required")
	}
	if strings.TrimSpace(normalized.TargetLanguage) == "" {
		return normalized, newAPIError(CodeParamError, "ast target language is required")
	}
	if normalized.SourceAudio.Format == "" {
		normalized.SourceAudio.Format = FormatWAV
	}
	if normalized.SourceAudio.Codec == "" {
		normalized.SourceAudio.Codec = "raw"
	}
	if normalized.SourceAudio.Rate == 0 {
		normalized.SourceAudio.Rate = SampleRate16000
	}
	if normalized.SourceAudio.Bits == 0 {
		normalized.SourceAudio.Bits = 16
	}
	if normalized.SourceAudio.Channel == 0 {
		normalized.SourceAudio.Channel = 1
	}
	if normalized.SourceAudio.Format != FormatWAV {
		return normalized, newAPIError(CodeParamError, "ast source audio format must be wav")
	}
	if normalized.SourceAudio.Rate != SampleRate16000 {
		return normalized, newAPIError(CodeParamError, "ast source audio rate must be 16000")
	}
	if normalized.SourceAudio.Bits != 16 {
		return normalized, newAPIError(CodeParamError, "ast source audio bits must be 16")
	}
	if normalized.SourceAudio.Channel != 1 {
		return normalized, newAPIError(CodeParamError, "ast source audio channel must be 1")
	}
	if normalized.Mode == ASTTranslateModeS2S {
		if normalized.TargetAudio.Format == "" {
			normalized.TargetAudio.Format = FormatOGG
		}
		if normalized.TargetAudio.Rate == 0 {
			normalized.TargetAudio.Rate = SampleRate48000
		}
		if normalized.TargetAudio.Channel == 0 {
			normalized.TargetAudio.Channel = 1
		}
		if normalized.TargetAudio.Format != FormatPCM && normalized.TargetAudio.Format != FormatOGG {
			return normalized, newAPIError(CodeParamError, "ast target audio format must be pcm or ogg_opus")
		}
	}
	if normalized.EventBuffer < 0 {
		return normalized, newAPIError(CodeParamError, "ast event buffer must be non-negative")
	}
	return normalized, nil
}

func resolvedASTTranslateEventBuffer(cfg ASTTranslateConfig) int {
	if cfg.EventBuffer > 0 {
		return cfg.EventBuffer
	}
	return defaultASTTranslateEventBuffer
}

func resolvedASTTranslateBackpressureTimeout(cfg ASTTranslateConfig) time.Duration {
	if cfg.BackpressureTimeout > 0 {
		return cfg.BackpressureTimeout
	}
	return defaultASTTranslateBackpressureTimeout
}

func shouldSendASTTargetAudio(cfg ASTTranslateConfig) bool {
	return cfg.Mode == ASTTranslateModeS2S || cfg.TargetAudio.Format != "" || cfg.TargetAudio.Rate != 0 || cfg.TargetAudio.Bits != 0 || cfg.TargetAudio.Channel != 0
}

func buildASTTranslateHeaders(creds auth.Credentials, resourceID string, connectID string) http.Header {
	headers := http.Header{}
	if creds.AccessKey != "" || creds.AccessToken != "" {
		if creds.AppID != "" {
			headers.Set("X-Api-App-Id", creds.AppID)
		}
		if creds.AppKey != "" && creds.AppKey != creds.AppID {
			headers.Set("X-Api-App-Key", creds.AppKey)
		}
		headers.Set("X-Api-Access-Key", firstNonEmpty(creds.AccessKey, creds.AccessToken))
	} else if creds.APIKey != "" {
		headers.Set("X-Api-Key", creds.APIKey)
	}
	if resourceID == "" {
		resourceID = creds.DefaultResourceID
	}
	if resourceID != "" {
		headers.Set("X-Api-Resource-Id", resourceID)
	}
	if connectID != "" {
		headers.Set("X-Api-Connect-Id", connectID)
	}
	return headers
}

func astUser(client *Client, user ASTUser) *astproto.User {
	uid := firstNonEmpty(user.UID, client.config.userID)
	return &astproto.User{
		UID:        uid,
		DID:        user.DID,
		Platform:   user.Platform,
		SDKVersion: user.SDKVersion,
		AppVersion: user.AppVersion,
	}
}

func astSourceAudio(audio ASTAudioConfig) *astproto.Audio {
	return &astproto.Audio{
		Format:  string(audio.Format),
		Codec:   audio.Codec,
		Rate:    int32(audio.Rate),
		Bits:    int32(audio.Bits),
		Channel: int32(audio.Channel),
	}
}

func astTargetAudio(audio ASTTargetAudioConfig) *astproto.Audio {
	return &astproto.Audio{
		Format:  string(audio.Format),
		Rate:    int32(audio.Rate),
		Bits:    int32(audio.Bits),
		Channel: int32(audio.Channel),
	}
}

func astCorpus(corpus *ASTTranslateCorpus) *astproto.Corpus {
	if corpus == nil {
		return nil
	}
	var correctWords string
	if len(corpus.CorrectWords) > 0 {
		if b, err := json.Marshal(corpus.CorrectWords); err == nil {
			correctWords = string(b)
		}
	}
	return &astproto.Corpus{
		BoostingTableID:       corpus.BoostingTableID,
		BoostingTableName:     corpus.BoostingTableName,
		HotWordsList:          append([]string(nil), corpus.HotWords...),
		GlossaryList:          cloneStringMap(corpus.Glossary),
		CorrectWords:          correctWords,
		RegexCorrectTableID:   corpus.RegexCorrectTableID,
		RegexCorrectTableName: corpus.RegexCorrectTableName,
		GlossaryTableID:       corpus.GlossaryTableID,
		GlossaryTableName:     corpus.GlossaryTableName,
	}
}

func astEventFromProto(resp *astproto.TranslateResponse) *ASTTranslateEvent {
	evt := &ASTTranslateEvent{
		Type:             ASTTranslateEventType(resp.Event),
		Text:             resp.Text,
		Audio:            copyBytes(resp.Data),
		StartTimeMS:      int(resp.StartTime),
		EndTimeMS:        int(resp.EndTime),
		SpeakerChanged:   resp.SpeakerChanged,
		DetectedLanguage: resp.DetectedLanguage,
		MutedDurationMS:  int(resp.MutedDuration),
	}
	if resp.ResponseMeta != nil {
		meta := resp.ResponseMeta
		evt.SessionID = meta.SessionID
		if meta.Billing != nil {
			evt.Usage = &ASTTranslateUsage{
				DurationMS: meta.Billing.DurationMsec,
				WordCount:  meta.Billing.WordCount,
			}
			for _, item := range meta.Billing.Items {
				evt.Usage.Items = append(evt.Usage.Items, ASTTranslateBillingItem{
					Unit:     item.Unit,
					Quantity: item.Quantity,
				})
			}
		}
		if meta.StatusCode != 0 && meta.StatusCode != astproto.StatusSuccess {
			msg := meta.Message
			if msg == "" {
				msg = "ast translate event error"
			}
			evt.Error = &Error{
				Code:    int(meta.StatusCode),
				Message: msg,
				ReqID:   meta.SessionID,
			}
		}
	}
	evt.IsFinal = evt.Type == ASTEventSessionFinished ||
		evt.Type == ASTEventSessionCanceled ||
		evt.Type == ASTEventSessionFailed
	return evt
}

func wrapASTTranslateEventError(eventType ASTTranslateEventType, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("ast translate event %d: %w", eventType, err)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
