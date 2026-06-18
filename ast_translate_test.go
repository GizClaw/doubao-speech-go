package doubaospeech

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/astproto"
	"github.com/gorilla/websocket"
)

func TestASTTranslateOpenSessionSendsStartRequest(t *testing.T) {
	client := NewClient("test-app", WithV2APIKey("test-ak", "test-app"), WithUserID("tester"))
	conn := newFakeWSConn()
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: "session-1", StatusCode: astproto.StatusSuccess},
		Event:        astproto.EventSessionStarted,
	}))

	svc := newASTTranslateService(client)
	dialer := &fakeDialer{conn: conn}
	svc.dialer = dialer

	cfg := DefaultASTTranslateConfig()
	cfg.SessionID = "session-1"
	session, err := svc.OpenSession(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	if got := dialer.url; !strings.HasSuffix(got, astTranslateEndpointPath) {
		t.Fatalf("url = %q, want suffix %q", got, astTranslateEndpointPath)
	}
	if got := dialer.headers.Get("X-Api-Resource-Id"); got != ResourceASTTranslate {
		t.Fatalf("resource header = %q, want %q", got, ResourceASTTranslate)
	}
	if got := dialer.headers.Get("X-Api-App-Id"); got != "test-app" {
		t.Fatalf("app id header = %q, want test-app", got)
	}
	if got := dialer.headers.Get("X-Api-Access-Key"); got != "test-ak" {
		t.Fatalf("access key header = %q, want test-ak", got)
	}

	writes := conn.writesSnapshot()
	if len(writes) == 0 {
		t.Fatalf("no writes sent")
	}
	req, err := astproto.UnmarshalRequest(writes[0])
	if err != nil {
		t.Fatalf("unmarshal start request: %v", err)
	}
	if req.Event != astproto.EventStartSession {
		t.Fatalf("event = %d, want start session", req.Event)
	}
	if req.RequestMeta == nil || req.RequestMeta.SessionID != "session-1" {
		t.Fatalf("request_meta = %+v, want session-1", req.RequestMeta)
	}
	if req.Request == nil || req.Request.Mode != "s2t" || req.Request.SourceLanguage != "zh" || req.Request.TargetLanguage != "en" {
		t.Fatalf("request params = %+v", req.Request)
	}
	if req.SourceAudio == nil || req.SourceAudio.Format != "wav" || req.SourceAudio.Rate != 16000 || req.SourceAudio.Bits != 16 || req.SourceAudio.Channel != 1 {
		t.Fatalf("source audio = %+v", req.SourceAudio)
	}
	if req.TargetAudio != nil {
		t.Fatalf("s2t default should not send target audio, got %+v", req.TargetAudio)
	}
	if req.Denoise == nil || *req.Denoise {
		t.Fatalf("denoise = %v, want explicit false", req.Denoise)
	}
}

func TestASTTranslateS2SSendsTargetAudio(t *testing.T) {
	client := NewClient("test-app", WithV2APIKey("test-ak", "test-app"), WithUserID("tester"))
	conn := newFakeWSConn()
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: "session-1", StatusCode: astproto.StatusSuccess},
		Event:        astproto.EventSessionStarted,
	}))

	svc := newASTTranslateService(client)
	svc.dialer = &fakeDialer{conn: conn}

	cfg := DefaultASTTranslateConfig()
	cfg.Mode = ASTTranslateModeS2S
	cfg.SessionID = "session-1"
	session, err := svc.OpenSession(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	req, err := astproto.UnmarshalRequest(conn.writesSnapshot()[0])
	if err != nil {
		t.Fatalf("unmarshal start request: %v", err)
	}
	if req.TargetAudio == nil {
		t.Fatalf("target audio is nil")
	}
	if req.TargetAudio.Format != "ogg_opus" || req.TargetAudio.Rate != 48000 || req.TargetAudio.Channel != 1 {
		t.Fatalf("target audio = %+v", req.TargetAudio)
	}
}

func TestASTTranslateStartRequestIncludesOptionalParams(t *testing.T) {
	client := NewClient("test-app", WithV2APIKey("test-ak", "test-app"), WithUserID("tester"))
	conn := newFakeWSConn()
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: "session-1", StatusCode: astproto.StatusSuccess},
		Event:        astproto.EventSessionStarted,
	}))

	svc := newASTTranslateService(client)
	svc.dialer = &fakeDialer{conn: conn}

	cfg := DefaultASTTranslateConfig()
	cfg.SessionID = "session-1"
	cfg.SpeakerID = "speaker-1"
	cfg.IsCustomSpeaker = true
	cfg.TTSResourceID = ResourceTTSV2
	cfg.SpeechRate = 20
	cfg.EnableSourceLanguageDetect = true

	session, err := svc.OpenSession(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	req, err := astproto.UnmarshalRequest(conn.writesSnapshot()[0])
	if err != nil {
		t.Fatalf("unmarshal start request: %v", err)
	}
	if req.Request.SpeakerID != "speaker-1" || req.Request.TTSResourceID != ResourceTTSV2 || req.Request.SpeechRate != 20 {
		t.Fatalf("request params = %+v", req.Request)
	}
	if req.Request.IsCustomSpeaker == nil || !*req.Request.IsCustomSpeaker {
		t.Fatalf("is_custom_speaker = %v, want true", req.Request.IsCustomSpeaker)
	}
	if req.Request.EnableSourceLanguageDetect == nil || !*req.Request.EnableSourceLanguageDetect {
		t.Fatalf("enable_source_language_detect = %v, want true", req.Request.EnableSourceLanguageDetect)
	}
}

func TestASTTranslateOpenSessionUnexpectedStartResponseClosesPromptly(t *testing.T) {
	client := NewClient("test-app", WithV2APIKey("test-ak", "test-app"), WithUserID("tester"))
	conn := newFakeWSConn()
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: "session-1", StatusCode: astproto.StatusSuccess},
		Event:        astproto.EventAudioMuted,
	}))

	svc := newASTTranslateService(client)
	svc.dialer = &fakeDialer{conn: conn}

	cfg := DefaultASTTranslateConfig()
	cfg.SessionID = "session-1"
	start := time.Now()
	_, err := svc.OpenSession(context.Background(), &cfg)
	if err == nil {
		t.Fatalf("OpenSession expected unexpected event error")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("OpenSession cleanup took %s, want prompt cleanup", elapsed)
	}
}

func TestASTTranslateSendAudioAndFinish(t *testing.T) {
	session, conn := newOpenedASTTranslateSessionForTest(t, nil)
	defer session.Close()

	if err := session.SendAudio(context.Background(), []byte{1, 2, 3}); err != nil {
		t.Fatalf("SendAudio error = %v", err)
	}
	if err := session.Finish(context.Background()); err != nil {
		t.Fatalf("Finish error = %v", err)
	}

	writes := conn.writesSnapshot()
	if len(writes) < 3 {
		t.Fatalf("writes count = %d, want >= 3", len(writes))
	}
	audioReq, err := astproto.UnmarshalRequest(writes[len(writes)-2])
	if err != nil {
		t.Fatalf("unmarshal audio request: %v", err)
	}
	if audioReq.Event != astproto.EventTaskRequest {
		t.Fatalf("audio event = %d, want task request", audioReq.Event)
	}
	if string(audioReq.SourceAudio.BinaryData) != string([]byte{1, 2, 3}) {
		t.Fatalf("audio bytes = %v", audioReq.SourceAudio.BinaryData)
	}

	finishReq, err := astproto.UnmarshalRequest(writes[len(writes)-1])
	if err != nil {
		t.Fatalf("unmarshal finish request: %v", err)
	}
	if finishReq.Event != astproto.EventFinishSession {
		t.Fatalf("finish event = %d, want finish session", finishReq.Event)
	}
}

func TestASTTranslateRecvEventsAndFailure(t *testing.T) {
	session, conn := newOpenedASTTranslateSessionForTest(t, nil)
	defer session.Close()

	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: session.SessionID(), StatusCode: astproto.StatusSuccess},
		Event:        astproto.EventTranslationSubtitleResponse,
		Text:         "hello",
	}))
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{
			SessionID:  session.SessionID(),
			StatusCode: astproto.StatusSuccess,
			Billing: &astproto.Billing{
				DurationMsec: 1200,
				Items: []astproto.BillingItem{{
					Unit:     "input_audio_tokens",
					Quantity: 1.5,
				}},
			},
		},
		Event:            astproto.EventSourceSubtitleEnd,
		Text:             "你好",
		StartTime:        10,
		EndTime:          1200,
		DetectedLanguage: "zh",
	}))
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: session.SessionID(), StatusCode: 45000001, Message: "bad request"},
		Event:        astproto.EventSessionFailed,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	evt, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("RecvEvent error = %v", err)
	}
	if evt.Type != ASTEventTranslationSubtitleResponse || evt.Text != "hello" {
		t.Fatalf("event = %+v", evt)
	}

	sourceEnd, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("RecvEvent source end error = %v", err)
	}
	if sourceEnd.Type != ASTEventSourceSubtitleEnd || sourceEnd.DetectedLanguage != "zh" || sourceEnd.StartTimeMS != 10 || sourceEnd.EndTimeMS != 1200 {
		t.Fatalf("source end event = %+v", sourceEnd)
	}
	if sourceEnd.Usage == nil || sourceEnd.Usage.DurationMS != 1200 || len(sourceEnd.Usage.Items) != 1 || sourceEnd.Usage.Items[0].Unit != "input_audio_tokens" {
		t.Fatalf("usage = %+v", sourceEnd.Usage)
	}

	failedEvt, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("session failed event should be delivered before fatal error, got %v", err)
	}
	if failedEvt.Type != ASTEventSessionFailed || failedEvt.Error == nil {
		t.Fatalf("failed event = %+v", failedEvt)
	}

	_, err = session.RecvEvent(ctx)
	if err == nil {
		t.Fatalf("expected session failed error")
	}
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != 45000001 || apiErr.Message != "bad request" {
		t.Fatalf("api error = %+v", apiErr)
	}
}

func TestASTTranslateValidation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ASTTranslateConfig)
		want string
	}{
		{
			name: "invalid mode",
			edit: func(cfg *ASTTranslateConfig) { cfg.Mode = "bad" },
			want: "mode",
		},
		{
			name: "missing source language",
			edit: func(cfg *ASTTranslateConfig) { cfg.SourceLanguage = "" },
			want: "source language",
		},
		{
			name: "bad source rate",
			edit: func(cfg *ASTTranslateConfig) { cfg.SourceAudio.Rate = SampleRate24000 },
			want: "rate",
		},
		{
			name: "bad source channel",
			edit: func(cfg *ASTTranslateConfig) { cfg.SourceAudio.Channel = 2 },
			want: "channel",
		},
		{
			name: "bad target format",
			edit: func(cfg *ASTTranslateConfig) {
				cfg.Mode = ASTTranslateModeS2S
				cfg.TargetAudio.Format = FormatMP3
			},
			want: "target audio format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultASTTranslateConfig()
			tc.edit(&cfg)
			_, err := normalizeASTTranslateConfig(&cfg)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestASTTranslateCloseIdempotentAndRaceSafe(t *testing.T) {
	session, _ := newOpenedASTTranslateSessionForTest(t, nil)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_ = session.Close()
		})
	}

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("concurrent Close did not finish")
	}
}

func newOpenedASTTranslateSessionForTest(t *testing.T, cfg *ASTTranslateConfig) (*ASTTranslateSession, *fakeWSConn) {
	t.Helper()

	client := NewClient("test-app", WithV2APIKey("test-ak", "test-app"), WithUserID("tester"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}
	svc := newASTTranslateService(client)
	svc.dialer = dialer

	sessionID := "session-1"
	if cfg != nil && cfg.SessionID != "" {
		sessionID = cfg.SessionID
	}
	conn.enqueue(websocket.BinaryMessage, mustBuildASTResponse(t, &astproto.TranslateResponse{
		ResponseMeta: &astproto.ResponseMeta{SessionID: sessionID, StatusCode: astproto.StatusSuccess},
		Event:        astproto.EventSessionStarted,
	}))

	if cfg == nil {
		defaultCfg := DefaultASTTranslateConfig()
		defaultCfg.SessionID = sessionID
		cfg = &defaultCfg
	}

	session, err := svc.OpenSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	return session, conn
}

func mustBuildASTResponse(t *testing.T, resp *astproto.TranslateResponse) []byte {
	t.Helper()
	payload, err := astproto.MarshalResponse(resp)
	if err != nil {
		t.Fatalf("MarshalResponse error = %v", err)
	}
	return payload
}
