package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestRealtimeDuplexOpenSessionSendsCreate(t *testing.T) {
	client := NewClient(WithAPIKey("test-key"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{
		conn: conn,
		resp: &http.Response{Header: http.Header{"X-Tt-Logid": []string{"log-1"}}},
	}

	svc := newRealtimeDuplexService(client)
	svc.dialer = dialer
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"session.created","event_id":"evt-1","session":{"id":"session-from-server"}}`))

	session, err := svc.OpenSession(context.Background(), &RealtimeDuplexConfig{
		Session: RealtimeDuplexSessionConfig{
			Model:        RealtimeDuplexModelDefault,
			Instructions: "be concise",
			Audio: RealtimeDuplexAudioConfig{
				Input: RealtimeDuplexAudioInputConfig{
					Format: RealtimeDuplexAudioFormat{Type: RealtimeDuplexAudioPCM, Rate: 16000},
				},
				Output: RealtimeDuplexAudioOutputConfig{
					Format: RealtimeDuplexAudioFormat{Type: RealtimeDuplexAudioPCMS16LE, Rate: 24000},
					Voice:  "zh_male_xiaotian_jupiter_bigtts",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	if !strings.HasSuffix(dialer.url, realtimeDuplexEndpointPath) {
		t.Fatalf("dial url = %s, want suffix %s", dialer.url, realtimeDuplexEndpointPath)
	}
	if got := dialer.headers.Get("X-Api-Key"); got != "test-key" {
		t.Fatalf("X-Api-Key = %q, want test-key", got)
	}
	if got := session.SessionID(); got != "session-from-server" {
		t.Fatalf("SessionID = %q, want session-from-server", got)
	}
	if got := session.LogID(); got != "log-1" {
		t.Fatalf("LogID = %q, want log-1", got)
	}

	writes := conn.writesSnapshot()
	if len(writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writes))
	}

	var event struct {
		Type    string `json:"type"`
		EventID string `json:"event_id"`
		Session struct {
			ID           string `json:"id"`
			Model        string `json:"model"`
			Instructions string `json:"instructions"`
			Audio        struct {
				Input struct {
					Format RealtimeDuplexAudioFormat `json:"format"`
				} `json:"input"`
				Output struct {
					Format RealtimeDuplexAudioFormat `json:"format"`
					Voice  string                    `json:"voice"`
				} `json:"output"`
			} `json:"audio"`
		} `json:"session"`
	}
	if err := json.Unmarshal(writes[0], &event); err != nil {
		t.Fatalf("unmarshal session.create: %v", err)
	}
	if event.Type != RealtimeDuplexEventSessionCreate {
		t.Fatalf("event type = %q, want %s", event.Type, RealtimeDuplexEventSessionCreate)
	}
	if event.EventID == "" {
		t.Fatalf("event_id is empty")
	}
	if event.Session.Model != RealtimeDuplexModelDefault {
		t.Fatalf("model = %q, want %s", event.Session.Model, RealtimeDuplexModelDefault)
	}
	if event.Session.Audio.Input.Format.Type != RealtimeDuplexAudioPCM {
		t.Fatalf("input format = %q, want %s", event.Session.Audio.Input.Format.Type, RealtimeDuplexAudioPCM)
	}
	if event.Session.Audio.Output.Format.Type != RealtimeDuplexAudioPCMS16LE {
		t.Fatalf("output format = %q, want %s", event.Session.Audio.Output.Format.Type, RealtimeDuplexAudioPCMS16LE)
	}
}

func TestRealtimeDuplexOpenSessionSendsAccessKeyHeaders(t *testing.T) {
	client := NewClient(WithAppID("test-app", "test-ak", AppKeyRealtime))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}

	svc := newRealtimeDuplexService(client)
	svc.dialer = dialer
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"session.created","event_id":"evt-1","session":{"id":"session-from-server"}}`))

	session, err := svc.OpenSession(context.Background(), nil)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	if got := dialer.headers.Get("X-Api-Access-Key"); got != "test-ak" {
		t.Fatalf("X-Api-Access-Key = %q, want test-ak", got)
	}
	if got := dialer.headers.Get("X-Api-App-Id"); got != "test-app" {
		t.Fatalf("X-Api-App-Id = %q, want test-app", got)
	}
	if got := dialer.headers.Get("X-Api-App-Key"); got != AppKeyRealtime {
		t.Fatalf("X-Api-App-Key = %q, want %s", got, AppKeyRealtime)
	}
	if got := dialer.headers.Get("X-Api-Key"); got != "" {
		t.Fatalf("X-Api-Key = %q, want empty", got)
	}
}

func TestRealtimeDuplexSendEvents(t *testing.T) {
	session, conn := newOpenedRealtimeDuplexSessionForTest(t)

	if err := session.SendAudio(context.Background(), []byte{1, 2, 3}); err != nil {
		t.Fatalf("SendAudio error = %v", err)
	}
	if err := session.CommitAudio(context.Background()); err != nil {
		t.Fatalf("CommitAudio error = %v", err)
	}
	if err := session.SendSpeechText(context.Background(), RealtimeDuplexSpeechTextRequest{SpeechID: "speech-1", Text: "hello"}); err != nil {
		t.Fatalf("SendSpeechText error = %v", err)
	}
	if err := session.AppendReplacementSpeechText(context.Background(), RealtimeDuplexSpeechTextRequest{SpeechID: "speech-2", Text: "replace"}); err != nil {
		t.Fatalf("AppendReplacementSpeechText error = %v", err)
	}
	if err := session.CommitReplacementSpeechText(context.Background(), RealtimeDuplexSpeechTextRequest{SpeechID: "speech-2", Text: "done"}); err != nil {
		t.Fatalf("CommitReplacementSpeechText error = %v", err)
	}
	if err := session.CreateConversationItems(context.Background(), RealtimeDuplexConversationItem{
		Type: "message",
		Role: RealtimeDuplexRoleUser,
		Content: []RealtimeDuplexConversationContent{
			{Type: "input_text", Text: "context"},
		},
	}); err != nil {
		t.Fatalf("CreateConversationItems error = %v", err)
	}
	if err := session.UpdateConversationItems(context.Background(), RealtimeDuplexConversationItem{ID: "item-1", Type: "message"}); err != nil {
		t.Fatalf("UpdateConversationItems error = %v", err)
	}
	if err := session.RetrieveConversationItems(context.Background(), RealtimeDuplexConversationItem{ID: "item-1"}); err != nil {
		t.Fatalf("RetrieveConversationItems error = %v", err)
	}
	if err := session.DeleteConversationItems(context.Background(), RealtimeDuplexConversationItem{ID: "item-1"}); err != nil {
		t.Fatalf("DeleteConversationItems error = %v", err)
	}
	if err := session.CancelResponse(context.Background()); err != nil {
		t.Fatalf("CancelResponse error = %v", err)
	}
	if err := session.SendFunctionCallOutputs(context.Background(), RealtimeDuplexFunctionCallOutput{
		CallID: "call-1",
		Output: "tool output",
	}); err != nil {
		t.Fatalf("SendFunctionCallOutputs error = %v", err)
	}

	writes := conn.writesSnapshot()
	if len(writes) != 12 {
		t.Fatalf("writes = %d, want 12", len(writes))
	}

	assertEventType(t, writes[1], RealtimeDuplexEventInputAudioBufferAppend)
	var audio struct {
		Audio string `json:"audio"`
	}
	if err := json.Unmarshal(writes[1], &audio); err != nil {
		t.Fatalf("unmarshal audio append: %v", err)
	}
	if got := audio.Audio; got != base64.StdEncoding.EncodeToString([]byte{1, 2, 3}) {
		t.Fatalf("audio = %q", got)
	}

	assertEventType(t, writes[2], RealtimeDuplexEventInputAudioBufferCommit)
	assertEventType(t, writes[3], RealtimeDuplexEventSpeechTextBufferCommit)
	assertEventType(t, writes[4], RealtimeDuplexEventSpeechTextBufferReplacementAppend)
	assertEventType(t, writes[5], RealtimeDuplexEventSpeechTextBufferReplacementCommit)
	assertEventType(t, writes[6], RealtimeDuplexEventConversationItemCreate)
	assertEventType(t, writes[7], RealtimeDuplexEventConversationItemUpdate)
	assertEventType(t, writes[8], RealtimeDuplexEventConversationItemRetrieve)
	assertEventType(t, writes[9], RealtimeDuplexEventConversationItemDelete)
	assertEventType(t, writes[10], RealtimeDuplexEventResponseCancel)
	assertEventType(t, writes[11], RealtimeDuplexEventConversationItemCreate)

	var fc struct {
		Items []RealtimeDuplexConversationItem `json:"items"`
	}
	if err := json.Unmarshal(writes[11], &fc); err != nil {
		t.Fatalf("unmarshal function output: %v", err)
	}
	if len(fc.Items) != 1 || fc.Items[0].CallID != "call-1" || fc.Items[0].Role != RealtimeDuplexRoleTool {
		t.Fatalf("function output item = %+v", fc.Items)
	}
}

func TestRealtimeDuplexRecvEvent(t *testing.T) {
	session, conn := newOpenedRealtimeDuplexSessionForTest(t)
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"response.output_audio.delta","event_id":"evt-2","response_id":"resp-1","delta":"AQID"}`))
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"response.function_call_arguments.done","event_id":"evt-3","items":[{"call_id":"call-1","name":"lookup","arguments":"{\"q\":\"x\"}"}]}`))
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"vendor.new_event","event_id":"evt-4","custom":true}`))

	evt, err := session.RecvEvent(context.Background())
	if err != nil {
		t.Fatalf("RecvEvent audio error = %v", err)
	}
	if evt.Type != RealtimeDuplexEventResponseOutputAudioDelta || string(evt.Audio) != "\x01\x02\x03" {
		t.Fatalf("audio event = %+v", evt)
	}

	evt, err = session.RecvEvent(context.Background())
	if err != nil {
		t.Fatalf("RecvEvent function call error = %v", err)
	}
	if len(evt.FunctionCalls) != 1 || evt.FunctionCalls[0].CallID != "call-1" {
		t.Fatalf("function calls = %+v", evt.FunctionCalls)
	}

	evt, err = session.RecvEvent(context.Background())
	if err != nil {
		t.Fatalf("RecvEvent unknown error = %v", err)
	}
	if evt.Type != "vendor.new_event" || len(evt.Raw) == 0 {
		t.Fatalf("unknown event = %+v", evt)
	}
}

func TestRealtimeDuplexErrorEvent(t *testing.T) {
	session, conn := newOpenedRealtimeDuplexSessionForTest(t)
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"error","event_id":"evt-2","error":{"type":"invalid_request","code":"3001","message":"bad input","event_id":"req-1"}}`))

	evt, err := session.RecvEvent(context.Background())
	if err == nil {
		t.Fatalf("RecvEvent expected error")
	}
	if evt == nil || evt.Error == nil {
		t.Fatalf("event error missing: %+v", evt)
	}
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.Code != CodeParamError || apiErr.Message != "bad input" || apiErr.ReqID != "req-1" {
		t.Fatalf("api error = %+v", apiErr)
	}
}

func TestRealtimeDuplexCloseIdempotent(t *testing.T) {
	session, conn := newOpenedRealtimeDuplexSessionForTest(t)

	if err := session.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("second Close error = %v", err)
	}

	writes := conn.writesSnapshot()
	if len(writes) != 2 {
		t.Fatalf("writes = %d, want session.create + session.close", len(writes))
	}
	assertEventType(t, writes[1], RealtimeDuplexEventSessionClose)
}

func newOpenedRealtimeDuplexSessionForTest(t *testing.T) (*RealtimeDuplexSession, *fakeWSConn) {
	t.Helper()

	client := NewClient(WithAPIKey("test-key"))
	conn := newFakeWSConn()
	svc := newRealtimeDuplexService(client)
	svc.dialer = &fakeDialer{conn: conn}
	conn.enqueue(websocket.TextMessage, []byte(`{"type":"session.created","event_id":"evt-1","session":{"id":"session-1"}}`))

	session, err := svc.OpenSession(context.Background(), nil)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	return session, conn
}

func assertEventType(t *testing.T, payload []byte, want string) {
	t.Helper()
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Type != want {
		t.Fatalf("event type = %q, want %s; payload=%s", event.Type, want, payload)
	}
}
