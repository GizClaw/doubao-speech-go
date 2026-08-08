package doubaospeech

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/gorilla/websocket"
)

type realtimeIntegrationCapture struct {
	headers      http.Header
	path         string
	startConn    *protocol.ParsedFrame
	startSession *protocol.ParsedFrame
	rag          *protocol.ParsedFrame
	err          error
}

func TestRealtimeProtocolIntegration(t *testing.T) {
	resultCh := make(chan realtimeIntegrationCapture, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture := realtimeIntegrationCapture{headers: r.Header.Clone(), path: r.URL.Path}
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			capture.err = fmt.Errorf("upgrade websocket: %w", err)
			resultCh <- capture
			return
		}
		defer conn.Close()

		readFrame := func() (*protocol.ParsedFrame, error) {
			messageType, payload, readErr := conn.ReadMessage()
			if readErr != nil {
				return nil, readErr
			}
			if messageType != websocket.BinaryMessage {
				return nil, fmt.Errorf("message type = %d, want binary", messageType)
			}
			return protocol.ParseServerFrame(payload)
		}
		writeEvent := func(event int32, sessionID, connectID string, payload []byte) error {
			frame, buildErr := protocol.BuildEventFrame(protocol.EventFrame{
				MessageType:   protocol.MessageTypeFullServer,
				Flags:         protocol.FlagWithEvent,
				Event:         event,
				SessionID:     sessionID,
				ConnectID:     connectID,
				Serialization: protocol.SerializationJSON,
				Compression:   protocol.CompressionNone,
				Payload:       payload,
			})
			if buildErr != nil {
				return buildErr
			}
			return conn.WriteMessage(websocket.BinaryMessage, frame)
		}

		capture.startConn, err = readFrame()
		if err == nil {
			err = writeEvent(protocol.EventConnectionStarted, "", "connect-server", []byte(`{}`))
		}
		if err == nil {
			capture.startSession, err = readFrame()
		}
		if err == nil {
			err = writeEvent(int32(EventSessionStarted), capture.startSession.SessionID, "", []byte(`{"dialog_id":"dialog-server"}`))
		}
		if err == nil {
			capture.rag, err = readFrame()
		}
		if err == nil {
			err = writeEvent(int32(EventChatResponse), capture.rag.SessionID, "", []byte(`{"content":"收到。","question_id":"q-1","reply_id":"r-1"}`))
		}
		capture.err = err
		resultCh <- capture
		if err == nil {
			<-release
		}
	}))
	defer func() {
		releaseOnce.Do(func() { close(release) })
		server.Close()
	}()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(
		"integration-app",
		WithAPIKey("integration-key"),
		WithResourceID("integration-resource"),
		WithWebSocketURL(wsURL),
	)
	cfg := DefaultRealtimeConfig()
	cfg.Model = RealtimeModelO20
	cfg.InputMode = RealtimeInputModePushToTalk
	cfg.TTS.Speaker = "integration-speaker"
	cfg.Instructions = "只回答收到"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	session, err := client.Realtime.OpenSession(ctx, &cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()
	if session.DialogID() != "dialog-server" {
		t.Fatalf("DialogID = %q, want dialog-server", session.DialogID())
	}
	if err := session.SendRAGText(ctx, `[{"title":"t","content":"c"}]`); err != nil {
		t.Fatalf("SendRAGText error = %v", err)
	}
	evt, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("RecvEvent error = %v", err)
	}
	if evt.Text != "收到。" || evt.QuestionID != "q-1" || evt.ReplyID != "r-1" {
		t.Fatalf("response event = %+v", evt)
	}

	var capture realtimeIntegrationCapture
	select {
	case capture = <-resultCh:
	case <-ctx.Done():
		t.Fatalf("wait server capture: %v", ctx.Err())
	}
	if capture.err != nil {
		t.Fatalf("server protocol error: %v", capture.err)
	}
	if capture.path != realtimeEndpointPath {
		t.Fatalf("websocket path = %q, want %q", capture.path, realtimeEndpointPath)
	}
	for name, want := range map[string]string{
		"X-Api-App-Id":      "integration-app",
		"X-Api-Key":         "integration-key",
		"X-Api-Resource-Id": "integration-resource",
	} {
		if got := capture.headers.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	connectID := capture.headers.Get("X-Api-Connect-Id")
	if connectID == "" || capture.headers.Get("X-Api-Request-Id") != connectID {
		t.Fatalf("connect/request IDs = %q/%q, want same non-empty value", connectID, capture.headers.Get("X-Api-Request-Id"))
	}

	if capture.startConn.Event != protocol.EventStartConnection || capture.startConn.SessionID != "" {
		t.Fatalf("start connection envelope = %+v", capture.startConn)
	}
	assertJSONContract(t, capture.startConn.Payload, `{}`)
	if capture.startSession.Event != realtimeStartSessionEvent || capture.startSession.SessionID == "" {
		t.Fatalf("start session envelope = %+v", capture.startSession)
	}
	assertJSONContract(t, capture.startSession.Payload, `{
		"asr":{"language":"zh-CN"},
		"tts":{"speaker":"integration-speaker","audio_config":{"channel":1,"format":"pcm_s16le","sample_rate":24000,"bits":16}},
		"dialog":{"system_role":"只回答收到","extra":{"input_mod":"push_to_talk","model":"1.2.1.1"}}
	}`)
	if capture.rag.Event != realtimeRAGTextEvent || capture.rag.SessionID != capture.startSession.SessionID {
		t.Fatalf("RAG envelope = %+v, start session ID = %q", capture.rag, capture.startSession.SessionID)
	}
	assertJSONContract(t, capture.rag.Payload, `{"external_rag":"[{\"title\":\"t\",\"content\":\"c\"}]"}`)

	releaseOnce.Do(func() { close(release) })
}
