package doubaospeech

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/gorilla/websocket"
)

func TestRealtimeOpenSessionSendsLifecycleFrames(t *testing.T) {
	client := NewClient(WithAppID("test-app", "test-ak", AppKeyRealtime), WithUserID("tester"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}

	svc := newRealtimeService(client)
	svc.dialer = dialer

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, protocol.EventConnectionStarted, "", "connect-1", []byte(`{"ok":true}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventSessionStarted), "session-from-server", "", []byte(`{"ok":true}`)))

	session, err := svc.OpenSession(context.Background(), nil)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	writes := conn.writesSnapshot()
	if len(writes) < 2 {
		t.Fatalf("writes count = %d, want >= 2", len(writes))
	}

	startConnFrame, err := protocol.ParseServerFrame(writes[0])
	if err != nil {
		t.Fatalf("parse start-connection frame: %v", err)
	}
	if !startConnFrame.HasEvent || startConnFrame.Event != protocol.EventStartConnection {
		t.Fatalf("start connection event = %d, want %d", startConnFrame.Event, protocol.EventStartConnection)
	}

	startSessionFrame, err := protocol.ParseServerFrame(writes[1])
	if err != nil {
		t.Fatalf("parse start-session frame: %v", err)
	}
	if !startSessionFrame.HasEvent || startSessionFrame.Event != realtimeStartSessionEvent {
		t.Fatalf("start session event = %d, want %d", startSessionFrame.Event, realtimeStartSessionEvent)
	}
	if startSessionFrame.SessionID == "" {
		t.Fatalf("start session frame should contain session ID")
	}
}

func TestRealtimeSendUserMessageIncludesUpdatedState(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	session.UpdateHistory([]RealtimeConversationMessage{{Role: "assistant", Content: "old answer"}})
	session.UpdatePrompt(RealtimePromptConfig{System: "you are concise", Variables: map[string]string{"style": "short"}})
	session.UpdateProps(RealtimeGenerationProps{Temperature: 0.3, TopP: 0.7, MaxTokens: 64})

	if err := session.SendUserMessage(context.Background(), "hello"); err != nil {
		t.Fatalf("SendUserMessage error = %v", err)
	}

	writes := conn.writesSnapshot()
	if len(writes) < 3 {
		t.Fatalf("writes count = %d, want >= 3", len(writes))
	}

	userFrame, err := protocol.ParseServerFrame(writes[len(writes)-1])
	if err != nil {
		t.Fatalf("parse user text frame: %v", err)
	}
	if userFrame.Event != realtimeUserTextEvent {
		t.Fatalf("event = %d, want %d", userFrame.Event, realtimeUserTextEvent)
	}

	var payload map[string]any
	if err := json.Unmarshal(userFrame.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["content"]; got != "hello" {
		t.Fatalf("content = %v, want hello", got)
	}
	if _, ok := payload["history"]; !ok {
		t.Fatalf("history is missing")
	}
	if _, ok := payload["prompt"]; !ok {
		t.Fatalf("prompt is missing")
	}
	if _, ok := payload["props"]; !ok {
		t.Fatalf("props is missing")
	}
}

func TestRealtimeStartPayloadIncludesTypedModeModelAndRates(t *testing.T) {
	cfg := DefaultRealtimeConfig()
	cfg.InputMode = RealtimeInputModePushToTalk
	cfg.Model = RealtimeModelVersion("O")
	cfg.TTS.AudioConfig.SpeechRate = 12
	cfg.TTS.AudioConfig.LoudnessRate = -5
	cfg.Dialog.Extra = map[string]any{
		"extra": map[string]any{
			"input_mod": "text",
			"model":     "2.2.0.0",
		},
	}
	cfg.TTS.Extra = map[string]any{
		"audio_config": map[string]any{
			"speech_rate": 99,
		},
	}

	normalized, err := normalizeRealtimeConfig(&cfg)
	if err != nil {
		t.Fatalf("normalizeRealtimeConfig error = %v", err)
	}
	payloadBytes, err := buildRealtimeStartPayload(normalized)
	if err != nil {
		t.Fatalf("buildRealtimeStartPayload error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	dialog := payload["dialog"].(map[string]any)
	dialogExtra := dialog["extra"].(map[string]any)
	if got := dialogExtra["input_mod"]; got != string(RealtimeInputModePushToTalk) {
		t.Fatalf("input_mod = %v, want %s", got, RealtimeInputModePushToTalk)
	}
	if got := dialogExtra["model"]; got != string(RealtimeModelO20) {
		t.Fatalf("model = %v, want %s", got, RealtimeModelO20)
	}

	tts := payload["tts"].(map[string]any)
	audioConfig := tts["audio_config"].(map[string]any)
	if got := audioConfig["channel"]; got != float64(1) {
		t.Fatalf("channel = %v, want preserved channel 1", got)
	}
	if got := audioConfig["format"]; got != string(FormatPCM) {
		t.Fatalf("format = %v, want preserved format %s", got, FormatPCM)
	}
	if got := audioConfig["speech_rate"]; got != float64(12) {
		t.Fatalf("speech_rate = %v, want 12", got)
	}
	if got := audioConfig["loudness_rate"]; got != float64(-5) {
		t.Fatalf("loudness_rate = %v, want -5", got)
	}
}

func TestRealtimeConfigValidationRejectsInvalidTypedFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  RealtimeConfig
		want string
	}{
		{
			name: "input mode",
			cfg: RealtimeConfig{
				TTS:       DefaultRealtimeConfig().TTS,
				InputMode: RealtimeInputMode("invalid"),
			},
			want: "unsupported realtime input mode",
		},
		{
			name: "model",
			cfg: RealtimeConfig{
				TTS:   DefaultRealtimeConfig().TTS,
				Model: RealtimeModelVersion("1"),
			},
			want: "unsupported realtime model",
		},
		{
			name: "speech rate",
			cfg: RealtimeConfig{
				TTS: RealtimeTTSConfig{
					Speaker: "zh_female_cancan",
					AudioConfig: RealtimeAudioConfig{
						Channel:    1,
						Format:     FormatPCM,
						SampleRate: SampleRate16000,
						Bits:       16,
						SpeechRate: 101,
					},
				},
			},
			want: "speech_rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeRealtimeConfig(&tt.cfg)
			if err == nil {
				t.Fatalf("normalizeRealtimeConfig expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRealtimeControlEventsUseUpdatedAPI(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	if err := session.EndASR(context.Background()); err != nil {
		t.Fatalf("EndASR error = %v", err)
	}
	if err := session.Interrupt(context.Background()); err != nil {
		t.Fatalf("Interrupt error = %v", err)
	}
	if err := session.FinishSession(context.Background()); err != nil {
		t.Fatalf("FinishSession error = %v", err)
	}

	writes := conn.writesSnapshot()
	if len(writes) < 5 {
		t.Fatalf("writes count = %d, want >= 5", len(writes))
	}

	wantEvents := []int32{realtimeEndASREvent, realtimeClientInterrupt, realtimeFinishSessionEvent}
	for i, want := range wantEvents {
		frame, err := protocol.ParseServerFrame(writes[len(writes)-len(wantEvents)+i])
		if err != nil {
			t.Fatalf("parse control frame %d: %v", i, err)
		}
		if frame.Event != want {
			t.Fatalf("control frame %d event = %d, want %d", i, frame.Event, want)
		}
	}
}

func TestRealtimeSendTTSTextUsesStartEndProtocol(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	if err := session.SendTTSText(context.Background(), " hello tts "); err != nil {
		t.Fatalf("SendTTSText error = %v", err)
	}

	writes := conn.writesSnapshot()
	if len(writes) < 3 {
		t.Fatalf("writes count = %d, want >= 3", len(writes))
	}

	firstFrame, err := protocol.ParseServerFrame(writes[len(writes)-3])
	if err != nil {
		t.Fatalf("parse first tts frame: %v", err)
	}
	contentFrame, err := protocol.ParseServerFrame(writes[len(writes)-2])
	if err != nil {
		t.Fatalf("parse content tts frame: %v", err)
	}
	lastFrame, err := protocol.ParseServerFrame(writes[len(writes)-1])
	if err != nil {
		t.Fatalf("parse last tts frame: %v", err)
	}
	if firstFrame.Event != realtimeTTSTextEvent || contentFrame.Event != realtimeTTSTextEvent || lastFrame.Event != realtimeTTSTextEvent {
		t.Fatalf("events = %d/%d/%d, want %d", firstFrame.Event, contentFrame.Event, lastFrame.Event, realtimeTTSTextEvent)
	}

	var firstPayload map[string]any
	if err := json.Unmarshal(firstFrame.Payload, &firstPayload); err != nil {
		t.Fatalf("unmarshal first payload: %v", err)
	}
	if firstPayload["start"] != true || firstPayload["end"] != false || firstPayload["content"] != "" {
		t.Fatalf("first payload = %#v, want start packet", firstPayload)
	}

	var contentPayload map[string]any
	if err := json.Unmarshal(contentFrame.Payload, &contentPayload); err != nil {
		t.Fatalf("unmarshal content payload: %v", err)
	}
	if contentPayload["start"] != false || contentPayload["end"] != false || contentPayload["content"] != " hello tts " {
		t.Fatalf("content payload = %#v, want content packet", contentPayload)
	}

	var lastPayload map[string]any
	if err := json.Unmarshal(lastFrame.Payload, &lastPayload); err != nil {
		t.Fatalf("unmarshal last payload: %v", err)
	}
	if lastPayload["start"] != false || lastPayload["end"] != true || lastPayload["content"] != "" {
		t.Fatalf("last payload = %#v, want end packet", lastPayload)
	}
}

func TestRealtimeDecodeUpdatedPayloadFields(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	payload := []byte(`{
		"content":"partial answer",
		"question_id":"q-1",
		"reply_id":"r-1",
		"tts_type":"default",
		"status_code":"20000002",
		"usage":{"input_text_tokens":1,"output_text_tokens":2},
		"results":[{"text":"recognized","is_interim":false}]
	}`)
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatResponse), session.SessionID(), "", payload))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	evt, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("RecvEvent error = %v", err)
	}
	if evt.QuestionID != "q-1" || evt.ReplyID != "r-1" || evt.TTSType != "default" || evt.StatusCode != "20000002" {
		t.Fatalf("event metadata = %+v", evt)
	}
	if evt.Usage == nil || evt.Usage.InputTextTokens != 1 || evt.Usage.OutputTextTokens != 2 {
		t.Fatalf("usage = %+v", evt.Usage)
	}
	if len(evt.Results) != 1 || evt.Results[0].Text != "recognized" || evt.Results[0].IsInterim {
		t.Fatalf("results = %+v", evt.Results)
	}
}

func TestRealtimeRecvFinalThenErrorOrder(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatEnded), session.SessionID(), "", []byte(`{"content":"done"}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerErrorFrame(t, int32(EventSessionFailed), session.SessionID(), 3005, []byte(`{"code":3005,"message":"boom","reqid":"req-1","trace_id":"trace-1","log_id":"log-1"}`)))

	seenFinal := false
	for evt, err := range session.Recv() {
		if err != nil {
			if !seenFinal {
				t.Fatalf("error arrived before final event: %v", err)
			}
			if !strings.Contains(err.Error(), "realtime event") {
				t.Fatalf("error = %v, want wrapped realtime event error", err)
			}
			apiErr, ok := AsError(err)
			if !ok {
				t.Fatalf("want *Error, got %T (%v)", err, err)
			}
			if apiErr.TraceID != "trace-1" {
				t.Fatalf("trace_id = %q, want %q", apiErr.TraceID, "trace-1")
			}
			if apiErr.LogID != "log-1" {
				t.Fatalf("log_id = %q, want %q", apiErr.LogID, "log-1")
			}
			if apiErr.ConnectID != session.conn.connectID {
				t.Fatalf("connect_id = %q, want %q", apiErr.ConnectID, session.conn.connectID)
			}
			return
		}

		if evt != nil && evt.Type == EventChatEnded {
			if !evt.IsFinal {
				t.Fatalf("chat-ended event should be final: %+v", evt)
			}
			seenFinal = true
		}
	}

	t.Fatalf("Recv ended unexpectedly without terminal error")
}

func TestRealtimeFinalDeliveredOncePerTurn(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatEnded), session.SessionID(), "", []byte(`{"content":"turn done"}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventTTSFinished), session.SessionID(), "", []byte(`{"content":"tts done"}`)))

	finalCount := 0
	seen := 0
	for evt, err := range session.Recv() {
		if err != nil {
			t.Fatalf("Recv error = %v", err)
		}
		seen++
		if evt.IsFinal {
			finalCount++
		}
		if seen == 2 {
			break
		}
	}

	if finalCount != 1 {
		t.Fatalf("final event count = %d, want 1", finalCount)
	}
}

func TestRealtimeSendAudioResetsFinalStateBetweenTurns(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := session.SendAudio(ctx, []byte{0x01, 0x02}); err != nil {
		t.Fatalf("round1 SendAudio error = %v", err)
	}
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventASREnded), session.SessionID(), "", []byte(`{"text":"round1"}`)))

	r1, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("round1 RecvEvent error = %v", err)
	}
	if r1 == nil || !r1.IsFinal {
		t.Fatalf("round1 event = %+v, want final event", r1)
	}

	if err := session.SendAudio(ctx, []byte{0x03, 0x04}); err != nil {
		t.Fatalf("round2 SendAudio error = %v", err)
	}
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventASREnded), session.SessionID(), "", []byte(`{"text":"round2"}`)))

	r2, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("round2 RecvEvent error = %v", err)
	}
	if r2 == nil || !r2.IsFinal {
		t.Fatalf("round2 event = %+v, want final event", r2)
	}
}

func TestRealtimeConcurrentRecvNotSupported(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	firstDone := make(chan error, 1)
	go func() {
		_, err := session.RecvEvent(context.Background())
		firstDone <- err
	}()

	time.Sleep(20 * time.Millisecond)

	_, err := session.RecvEvent(context.Background())
	if err == nil {
		t.Fatalf("second RecvEvent expected error")
	}
	if !strings.Contains(err.Error(), "concurrent Recv") {
		t.Fatalf("error = %v, want concurrent Recv message", err)
	}

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatResponse), session.SessionID(), "", []byte(`{"content":"ok"}`)))

	select {
	case recvErr := <-firstDone:
		if recvErr != nil {
			t.Fatalf("first RecvEvent error = %v", recvErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("first RecvEvent did not finish")
	}
}

func TestRealtimeBackpressureReturnsError(t *testing.T) {
	cfg := &RealtimeConfig{
		TTS: RealtimeTTSConfig{
			Speaker: "zh_female_cancan",
			AudioConfig: RealtimeAudioConfig{
				Channel:    1,
				Format:     FormatPCM,
				SampleRate: SampleRate16000,
				Bits:       16,
			},
		},
		EventBuffer:         1,
		BackpressureTimeout: 30 * time.Millisecond,
	}

	session, conn := newOpenedRealtimeSessionForTest(t, cfg)
	defer session.Close()

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatResponse), session.SessionID(), "", []byte(`{"content":"1"}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatResponse), session.SessionID(), "", []byte(`{"content":"2"}`)))

	time.Sleep(120 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	firstEvt, err := session.RecvEvent(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "buffer full") {
			t.Fatalf("first RecvEvent error = %v, want buffer full", err)
		}
		return
	}
	if firstEvt == nil {
		t.Fatalf("first RecvEvent got nil event and nil error")
	}

	_, err = session.RecvEvent(ctx)
	if err == nil {
		t.Fatalf("second RecvEvent expected backpressure error")
	}
	if !strings.Contains(err.Error(), "buffer full") {
		t.Fatalf("error = %v, want buffer full", err)
	}
}

func TestRealtimeCloseIdempotentAndRaceSafe(t *testing.T) {
	session, _ := newOpenedRealtimeSessionForTest(t, nil)

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

	if err := session.Close(); err != nil {
		t.Fatalf("Close after race error = %v", err)
	}
}

func TestRealtimeOpenCloseLoopCleansReceiveLoop(t *testing.T) {
	const loops = 20

	for i := range loops {
		session, _ := newOpenedRealtimeSessionForTest(t, nil)
		if err := session.Close(); err != nil {
			t.Fatalf("loop %d close error = %v", i, err)
		}

		select {
		case <-session.recvDone:
		case <-time.After(1 * time.Second):
			t.Fatalf("loop %d recv loop did not exit", i)
		}
	}
}

func newOpenedRealtimeSessionForTest(t *testing.T, cfg *RealtimeConfig) (*RealtimeSession, *fakeWSConn) {
	t.Helper()

	client := NewClient(WithAppID("test-app", "test-ak", AppKeyRealtime), WithUserID("tester"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}

	svc := newRealtimeService(client)
	svc.dialer = dialer

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, protocol.EventConnectionStarted, "", "connect-1", []byte(`{"ok":true}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventSessionStarted), "session-1", "", []byte(`{"ok":true}`)))

	session, err := svc.OpenSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}

	return session, conn
}

func mustBuildRealtimeServerEventFrame(t *testing.T, event int32, sessionID, connectID string, payload []byte) []byte {
	t.Helper()

	raw, err := protocol.BuildEventFrame(protocol.EventFrame{
		MessageType:   protocol.MessageTypeFullServer,
		Flags:         protocol.FlagWithEvent,
		Event:         event,
		SessionID:     sessionID,
		ConnectID:     connectID,
		Serialization: protocol.SerializationJSON,
		Compression:   protocol.CompressionNone,
		Payload:       payload,
	})
	if err != nil {
		t.Fatalf("BuildEventFrame error = %v", err)
	}
	return raw
}

func mustBuildRealtimeServerErrorFrame(t *testing.T, event int32, sessionID string, code uint32, payload []byte) []byte {
	t.Helper()

	raw, err := protocol.BuildEventFrame(protocol.EventFrame{
		MessageType:   protocol.MessageTypeError,
		Flags:         protocol.FlagWithEvent,
		Event:         event,
		SessionID:     sessionID,
		ErrorCode:     code,
		Serialization: protocol.SerializationJSON,
		Compression:   protocol.CompressionNone,
		Payload:       payload,
	})
	if err != nil {
		t.Fatalf("BuildEventFrame error = %v", err)
	}
	return raw
}
