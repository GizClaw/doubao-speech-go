package doubaospeech

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/gorilla/websocket"
)

func TestPodcastOpenSessionAndReceiveRounds(t *testing.T) {
	client := NewClient("app-test", WithAccessKey("access-test"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}
	service := newPodcastService(client)
	service.dialer = dialer

	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, protocol.EventConnectionStarted, "", []byte(`{}`)))
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, podcastSessionStartedEvent, "session-test", []byte(`{"task_id":"server-task"}`)))

	head := true
	session, err := service.OpenSession(context.Background(), &PodcastRequest{
		SessionID:    "session-test",
		InputID:      "input-test",
		PromptText:   "Discuss the moon",
		UseHeadMusic: &head,
	})
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()
	if got := session.TaskID(); got != "server-task" {
		t.Fatalf("TaskID = %q, want server-task", got)
	}

	if got := dialer.headers.Get("X-Api-App-Id"); got != "app-test" {
		t.Fatalf("X-Api-App-Id = %q, want app-test", got)
	}
	if got := dialer.headers.Get("X-Api-App-Key"); got != podcastDefaultAppKey {
		t.Fatalf("X-Api-App-Key = %q, want %q", got, podcastDefaultAppKey)
	}
	if got := dialer.headers.Get("X-Api-Access-Key"); got != "access-test" {
		t.Fatalf("X-Api-Access-Key = %q, want access-test", got)
	}
	if got := dialer.headers.Get("X-Api-Resource-Id"); got != ResourcePodcast {
		t.Fatalf("X-Api-Resource-Id = %q, want %q", got, ResourcePodcast)
	}

	writes := conn.writesSnapshot()
	if len(writes) != 3 {
		t.Fatalf("writes = %d, want 3", len(writes))
	}
	start, err := protocol.ParseServerFrame(writes[1])
	if err != nil {
		t.Fatalf("parse StartSession = %v", err)
	}
	var payload PodcastRequest
	if err := json.Unmarshal(start.Payload, &payload); err != nil {
		t.Fatalf("decode StartSession payload = %v", err)
	}
	if payload.InputID != "input-test" || payload.Action == nil || *payload.Action != 4 {
		t.Fatalf("unexpected StartSession payload: %#v", payload)
	}

	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, int32(PodcastEventRoundStarted), "session-test", []byte(`{"round_id":2}`)))
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeAudioOnlyServer, int32(PodcastEventAudio), "session-test", []byte("audio")))
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, int32(PodcastEventRoundFinished), "session-test", []byte(`{"is_error":false}`)))
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, int32(PodcastEventSessionFinished), "session-test", []byte(`{}`)))

	round, err := session.RecvEvent(context.Background())
	if err != nil || round.Type != PodcastEventRoundStarted || round.RoundID != 2 {
		t.Fatalf("round event = %#v, err = %v", round, err)
	}
	audio, err := session.RecvEvent(context.Background())
	if err != nil || audio.Type != PodcastEventAudio || !bytes.Equal(audio.Audio, []byte("audio")) {
		t.Fatalf("audio event = %#v, err = %v", audio, err)
	}
	finished, err := session.RecvEvent(context.Background())
	if err != nil || finished.Type != PodcastEventRoundFinished || finished.IsError {
		t.Fatalf("round-finished event = %#v, err = %v", finished, err)
	}
	done, err := session.RecvEvent(context.Background())
	if err != nil || done.Type != PodcastEventSessionFinished {
		t.Fatalf("session-finished event = %#v, err = %v", done, err)
	}
}

func TestPodcastRetryPayloadAndCustomAppKey(t *testing.T) {
	client := NewClient("app-test", WithAppKey("app-key-test"), WithAccessKey("access-test"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}
	service := newPodcastService(client)
	service.dialer = dialer
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, protocol.EventConnectionStarted, "", []byte(`{}`)))
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, podcastSessionStartedEvent, "new-session", []byte(`{}`)))

	session, err := service.OpenSession(context.Background(), &PodcastRequest{
		SessionID:  "new-session",
		InputID:    "input-test",
		PromptText: "Resume",
		RetryInfo:  &PodcastRetryInfo{TaskID: "old-task", LastFinishedRoundID: 7},
	})
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()
	if got := dialer.headers.Get("X-Api-App-Key"); got != "app-key-test" {
		t.Fatalf("X-Api-App-Key = %q, want app-key-test", got)
	}
	start, err := protocol.ParseServerFrame(conn.writesSnapshot()[1])
	if err != nil {
		t.Fatal(err)
	}
	var payload PodcastRequest
	if err := json.Unmarshal(start.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RetryInfo == nil || payload.RetryInfo.TaskID != "old-task" || payload.RetryInfo.LastFinishedRoundID != 7 {
		t.Fatalf("retry_info = %#v", payload.RetryInfo)
	}
}

func TestPodcastRequiresAccessKeyBeforeDial(t *testing.T) {
	service := newPodcastService(NewClient("app-test"))
	_, err := service.OpenSession(context.Background(), &PodcastRequest{InputID: "input", PromptText: "prompt"})
	if err == nil {
		t.Fatal("OpenSession accepted an empty access key")
	}
	apiErr, ok := AsError(err)
	if !ok || !apiErr.IsAuthError() {
		t.Fatalf("error = %T %v, want auth Error", err, err)
	}
}

func TestPodcastAcceptsAPIKeyAuthentication(t *testing.T) {
	client := NewClient("", WithAPIKey("api-key-test"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}
	service := newPodcastService(client)
	service.dialer = dialer
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, protocol.EventConnectionStarted, "", []byte(`{}`)))
	conn.enqueue(websocket.BinaryMessage, podcastServerEvent(t, protocol.MessageTypeFullServer, podcastSessionStartedEvent, "session-test", []byte(`{}`)))
	session, err := service.OpenSession(context.Background(), &PodcastRequest{SessionID: "session-test", InputID: "input", PromptText: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := dialer.headers.Get("X-Api-Key"); got != "api-key-test" {
		t.Fatalf("X-Api-Key = %q, want api-key-test", got)
	}
	if got := dialer.headers.Get("X-Api-Access-Key"); got != "" {
		t.Fatalf("X-Api-Access-Key = %q, want empty", got)
	}
}

func TestPodcastWriteHonorsCanceledContext(t *testing.T) {
	conn := &blockingWriteWSConn{fakeWSConn: newFakeWSConn(), started: make(chan struct{})}
	session := &PodcastSession{conn: conn, sessionID: "session", taskID: "task"}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- session.writeEvent(ctx, podcastStartSessionEvent, "session", []byte(`{}`)) }()
	<-conn.started
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("write error = %v, want context.Canceled", err)
	}
}

type blockingWriteWSConn struct {
	*fakeWSConn
	started chan struct{}
}

func (c *blockingWriteWSConn) WriteMessage(_ int, _ []byte) error {
	close(c.started)
	<-c.reads
	return nil
}

func podcastServerEvent(t *testing.T, messageType protocol.MessageType, event int32, sessionID string, payload []byte) []byte {
	t.Helper()
	frame, err := protocol.BuildEventFrame(protocol.EventFrame{
		MessageType:   messageType,
		Event:         event,
		SessionID:     sessionID,
		ConnectID:     sessionID,
		Serialization: protocol.SerializationJSON,
		Payload:       payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}
