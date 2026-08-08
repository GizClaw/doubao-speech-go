package doubaospeech

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/gorilla/websocket"
)

const realtimeTestSpeaker = "test-speaker"

func TestRealtimeOpenSessionSendsLifecycleFrames(t *testing.T) {
	client := NewClient(
		"test-app",
		WithAPIKey("key-test"),
		WithResourceID("resource-test"),
		WithUserID("tester"),
	)
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}

	svc := newRealtimeService(client)
	svc.dialer = dialer

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, protocol.EventConnectionStarted, "", "connect-1", []byte(`{"ok":true}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventSessionStarted), "session-from-server", "", []byte(`{"ok":true}`)))

	cfg := DefaultRealtimeConfig()
	cfg.TTS.Speaker = realtimeTestSpeaker
	cfg.Model = RealtimeModelO20
	session, err := svc.OpenSession(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("OpenSession error = %v", err)
	}
	defer session.Close()

	if got := dialer.headers.Get("X-Api-App-Id"); got != "test-app" {
		t.Fatalf("X-Api-App-Id = %q, want test-app", got)
	}
	if got := dialer.headers.Get("X-Api-Key"); got != "key-test" {
		t.Fatalf("X-Api-Key = %q, want key-test", got)
	}
	if got := dialer.headers.Get("X-Api-Resource-Id"); got != "resource-test" {
		t.Fatalf("X-Api-Resource-Id = %q, want resource-test", got)
	}
	connectID := dialer.headers.Get("X-Api-Connect-Id")
	if connectID == "" {
		t.Fatal("X-Api-Connect-Id is empty")
	}
	if got := dialer.headers.Get("X-Api-Request-Id"); got != connectID {
		t.Fatalf("X-Api-Request-Id = %q, want connect ID %q", got, connectID)
	}
	if got := dialer.url; got != defaultWSURL+realtimeEndpointPath {
		t.Fatalf("websocket URL = %q, want %q", got, defaultWSURL+realtimeEndpointPath)
	}

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
	if startSessionFrame.ConnectID != "" {
		t.Fatalf("start session binary connect ID = %q, want empty", startSessionFrame.ConnectID)
	}
	var startPayload map[string]any
	if err := json.Unmarshal(startSessionFrame.Payload, &startPayload); err != nil {
		t.Fatalf("unmarshal start session payload: %v", err)
	}
	if _, ok := startPayload["session_id"]; ok {
		t.Fatalf("start session payload leaked binary session ID: %s", startSessionFrame.Payload)
	}
	if _, ok := startPayload["resource_id"]; ok {
		t.Fatalf("start session payload leaked handshake resource ID: %s", startSessionFrame.Payload)
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
	cfg.TTS.Speaker = realtimeTestSpeaker
	cfg.InputMode = RealtimeInputModePushToTalk
	cfg.Model = RealtimeModelVersion("O")
	cfg.ASR.AudioInfo = &RealtimeASRAudioInfo{Format: FormatSpeechOpus, SampleRate: SampleRate16000, Channel: 1}
	cfg.ASR.Extra = &RealtimeASRExtra{
		EndSmoothWindowMS: 1500,
		EnableCustomVAD:   new(true),
		EnableASRTwopass:  new(false),
		Context: &RealtimeASRContext{
			Hotwords:     []RealtimeHotword{{Word: "豆包"}},
			CorrectWords: map[string]string{"火山": "火山引擎"},
		},
	}
	cfg.TTS.AudioConfig.SpeechRate = 12
	cfg.TTS.AudioConfig.LoudnessRate = -5
	cfg.TTS.Extra = &RealtimeTTSExtra{
		ExplicitDialect: "sichuan",
		AIGCMetadata: &RealtimeAIGCMetadata{
			Enable:          new(true),
			ContentProducer: "producer",
			ProduceID:       "produce-1",
		},
		TTS20Model: "expressive",
	}
	cfg.Dialog.DialogID = "dialog-1"
	cfg.Dialog.Location = &RealtimeLocation{City: "北京", CountryCode: "CN"}
	cfg.Dialog.DialogContext = []RealtimeDialogContextItem{{Role: "user", Text: "你好", Timestamp: 1}}
	cfg.Dialog.Extra = &RealtimeDialogExtra{
		StrictAudit:                new(false),
		AuditResponse:              "blocked",
		EnableVolcWebsearch:        new(true),
		VolcWebsearchType:          "web_agent",
		VolcWebsearchAPIKey:        "search-key",
		VolcWebsearchBotID:         "bot-1",
		VolcWebsearchResultCount:   3,
		EnableMusic:                new(false),
		EnableLoudnessNorm:         new(true),
		EnableConversationTruncate: new(true),
		EnableUserQueryExit:        new(true),
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

	asr := payload["asr"].(map[string]any)
	audioInfo := asr["audio_info"].(map[string]any)
	if got := audioInfo["format"]; got != "speech_opus" {
		t.Fatalf("asr audio format = %v, want speech_opus", got)
	}
	asrExtra := asr["extra"].(map[string]any)
	if got := asrExtra["enable_asr_twopass"]; got != false {
		t.Fatalf("enable_asr_twopass = %v, want false", got)
	}
	asrContext := asrExtra["context"].(map[string]any)
	if _, ok := asrContext["hotwords"]; !ok {
		t.Fatalf("asr hotwords missing")
	}

	dialog := payload["dialog"].(map[string]any)
	if got := dialog["dialog_id"]; got != "dialog-1" {
		t.Fatalf("dialog_id = %v, want dialog-1", got)
	}
	location := dialog["location"].(map[string]any)
	if got := location["city"]; got != "北京" {
		t.Fatalf("location.city = %v, want 北京", got)
	}
	if _, ok := dialog["dialog_context"]; !ok {
		t.Fatalf("dialog_context missing")
	}
	dialogExtra := dialog["extra"].(map[string]any)
	if got := dialogExtra["input_mod"]; got != string(RealtimeInputModePushToTalk) {
		t.Fatalf("input_mod = %v, want %s", got, RealtimeInputModePushToTalk)
	}
	if got := dialogExtra["model"]; got != string(RealtimeModelO20) {
		t.Fatalf("model = %v, want %s", got, RealtimeModelO20)
	}
	if got := dialogExtra["strict_audit"]; got != false {
		t.Fatalf("strict_audit = %v, want false", got)
	}
	if got := dialogExtra["enable_volc_websearch"]; got != true {
		t.Fatalf("enable_volc_websearch = %v, want true", got)
	}

	tts := payload["tts"].(map[string]any)
	audioConfig := tts["audio_config"].(map[string]any)
	if got := audioConfig["channel"]; got != float64(1) {
		t.Fatalf("channel = %v, want preserved channel 1", got)
	}
	if got := audioConfig["format"]; got != string(FormatPCMS16LE) {
		t.Fatalf("format = %v, want preserved format %s", got, FormatPCMS16LE)
	}
	if got := audioConfig["speech_rate"]; got != float64(12) {
		t.Fatalf("speech_rate = %v, want 12", got)
	}
	if got := audioConfig["loudness_rate"]; got != float64(-5) {
		t.Fatalf("loudness_rate = %v, want -5", got)
	}
	ttsExtra := tts["extra"].(map[string]any)
	if got := ttsExtra["explicit_dialect"]; got != "sichuan" {
		t.Fatalf("explicit_dialect = %v, want sichuan", got)
	}
	if got := ttsExtra["tts_2.0_model"]; got != "expressive" {
		t.Fatalf("tts_2.0_model = %v, want expressive", got)
	}
}

func TestRealtimeStartPayloadFullWireContract(t *testing.T) {
	cfg := DefaultRealtimeConfig()
	cfg.InputMode = RealtimeInputModePushToTalk
	cfg.Model = RealtimeModelO20
	cfg.Instructions = "system instruction"
	cfg.ASR.AudioInfo = &RealtimeASRAudioInfo{
		Format:     FormatSpeechOpus,
		SampleRate: SampleRate16000,
		Channel:    1,
	}
	cfg.ASR.Extra = &RealtimeASRExtra{
		EndSmoothWindowMS:     1500,
		EnableCustomVAD:       new(true),
		EnableASRTwopass:      new(false),
		BoostingTableID:       "boost-id",
		BoostingTableName:     "boost-name",
		RegexCorrectTableID:   "regex-id",
		RegexCorrectTableName: "regex-name",
		Context: &RealtimeASRContext{
			Hotwords:     []RealtimeHotword{{Word: "豆包"}},
			CorrectWords: map[string]string{"火山": "火山引擎"},
		},
	}
	cfg.TTS = RealtimeTTSConfig{
		Speaker: "speaker-1",
		AudioConfig: RealtimeAudioConfig{
			Channel:      1,
			Format:       FormatPCMS16LE,
			SampleRate:   SampleRate24000,
			Bits:         16,
			SpeechRate:   12,
			LoudnessRate: -5,
		},
		Extra: &RealtimeTTSExtra{
			ExplicitDialect: "sichuan",
			AIGCMetadata: &RealtimeAIGCMetadata{
				Enable:            new(false),
				ContentProducer:   "producer",
				ProduceID:         "produce-id",
				ContentPropagator: "propagator",
				PropagateID:       "propagate-id",
			},
			TTS20Model: "expressive",
		},
	}
	cfg.Dialog = RealtimeDialogConfig{
		DialogID:      "dialog-1",
		BotName:       "bot",
		SystemRole:    "system instruction",
		SpeakingStyle: "concise",
		Location: &RealtimeLocation{
			Longitude:   116.4,
			Latitude:    39.9,
			City:        "北京",
			Country:     "中国",
			Province:    "北京",
			District:    "海淀",
			Town:        "中关村",
			CountryCode: "CN",
			Address:     "测试地址",
		},
		DialogContext: []RealtimeDialogContextItem{{Role: "user", Text: "你好", Timestamp: 1}},
		Extra: &RealtimeDialogExtra{
			StrictAudit:                  new(false),
			AuditResponse:                "blocked",
			EnableVolcWebsearch:          new(true),
			VolcWebsearchType:            "web_agent",
			VolcWebsearchAPIKey:          "search-key",
			VolcWebsearchBotID:           "bot-id",
			VolcWebsearchResultCount:     3,
			VolcWebsearchNoResultMessage: "no result",
			EnableMusic:                  new(false),
			EnableLoudnessNorm:           new(true),
			EnableConversationTruncate:   new(true),
			EnableUserQueryExit:          new(false),
		},
	}
	cfg.Prompt = RealtimePromptConfig{System: "compat prompt", Variables: map[string]string{"name": "豆包"}}
	cfg.Props = RealtimeGenerationProps{
		Temperature:      0.3,
		TopP:             0.7,
		MaxTokens:        64,
		PresencePenalty:  0.1,
		FrequencyPenalty: 0.2,
	}
	cfg.History = []RealtimeConversationMessage{{Role: "assistant", Content: "previous"}}

	normalized, err := normalizeRealtimeConfig(&cfg)
	if err != nil {
		t.Fatalf("normalizeRealtimeConfig error = %v", err)
	}
	payload, err := buildRealtimeStartPayload(normalized)
	if err != nil {
		t.Fatalf("buildRealtimeStartPayload error = %v", err)
	}

	assertJSONContract(t, payload, `{
		"asr": {
			"language": "zh-CN",
			"audio_info": {"format":"speech_opus","sample_rate":16000,"channel":1},
			"extra": {
				"end_smooth_window_ms":1500,
				"enable_custom_vad":true,
				"enable_asr_twopass":false,
				"boosting_table_id":"boost-id",
				"boosting_table_name":"boost-name",
				"regex_correct_table_id":"regex-id",
				"regex_correct_table_name":"regex-name",
				"context":{"hotwords":[{"word":"豆包"}],"correct_words":{"火山":"火山引擎"}}
			}
		},
		"tts": {
			"speaker":"speaker-1",
			"audio_config":{"channel":1,"format":"pcm_s16le","sample_rate":24000,"bits":16,"speech_rate":12,"loudness_rate":-5},
			"extra": {
				"explicit_dialect":"sichuan",
				"aigc_metadata":{"enable":false,"content_producer":"producer","produce_id":"produce-id","content_propagator":"propagator","propagate_id":"propagate-id"},
				"tts_2.0_model":"expressive"
			}
		},
		"dialog": {
			"dialog_id":"dialog-1",
			"bot_name":"bot",
			"system_role":"system instruction",
			"speaking_style":"concise",
			"location":{"longitude":116.4,"latitude":39.9,"city":"北京","country":"中国","province":"北京","district":"海淀","town":"中关村","country_code":"CN","address":"测试地址"},
			"dialog_context":[{"role":"user","text":"你好","timestamp":1}],
			"extra": {
				"strict_audit":false,
				"audit_response":"blocked",
				"enable_volc_websearch":true,
				"volc_websearch_type":"web_agent",
				"volc_websearch_api_key":"search-key",
				"volc_websearch_bot_id":"bot-id",
				"volc_websearch_result_count":3,
				"volc_websearch_no_result_message":"no result",
				"enable_music":false,
				"enable_loudness_norm":true,
				"enable_conversation_truncate":true,
				"enable_user_query_exit":false,
				"input_mod":"push_to_talk",
				"model":"1.2.1.1"
			}
		},
		"prompt":{"system":"compat prompt","variables":{"name":"豆包"}},
		"props":{"temperature":0.3,"top_p":0.7,"max_tokens":64,"presence_penalty":0.1,"frequency_penalty":0.2},
		"history":[{"role":"assistant","content":"previous"}]
	}`)
}

func TestRealtimeConfigsDoNotExposeRequestPassthroughMaps(t *testing.T) {
	assertNoAnyMapFields(t, reflect.TypeFor[RealtimeConfig](), "RealtimeConfig", map[reflect.Type]bool{})
	assertNoAnyMapFields(t, reflect.TypeFor[RealtimeDuplexConfig](), "RealtimeDuplexConfig", map[reflect.Type]bool{})
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
				TTS:       RealtimeTTSConfig{Speaker: realtimeTestSpeaker},
				InputMode: RealtimeInputMode("invalid"),
			},
			want: "unsupported realtime input mode",
		},
		{
			name: "model",
			cfg: RealtimeConfig{
				TTS:   RealtimeTTSConfig{Speaker: realtimeTestSpeaker},
				Model: RealtimeModelVersion("1"),
			},
			want: "unsupported realtime model",
		},
		{
			name: "speech rate",
			cfg: RealtimeConfig{
				Model: RealtimeModelO20,
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
	cfg := DefaultRealtimeConfig()
	cfg.InputMode = RealtimeInputModePushToTalk
	session, conn := newOpenedRealtimeSessionForTest(t, &cfg)
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

func TestRealtimeDocumentedSessionEvents(t *testing.T) {
	cfg := DefaultRealtimeConfig()
	cfg.Dialog.Extra = &RealtimeDialogExtra{EnableConversationTruncate: new(true)}
	session, conn := newOpenedRealtimeSessionForTest(t, &cfg)
	defer session.Close()

	if err := session.UpdateConfig(context.Background(), RealtimeUpdateConfig{
		TTS: &RealtimeTTSConfig{
			Speaker: "zh_female_vv_jupiter_bigtts",
			AudioConfig: RealtimeAudioConfig{
				SpeechRate:   5,
				LoudnessRate: 6,
			},
		},
		Dialog: &RealtimeDialogConfig{
			DialogID: "dialog-1",
			Location: &RealtimeLocation{City: "上海"},
		},
	}); err != nil {
		t.Fatalf("UpdateConfig error = %v", err)
	}
	if err := session.SendRAGText(context.Background(), `[{"title":"t","content":"c"}]`); err != nil {
		t.Fatalf("SendRAGText error = %v", err)
	}
	if err := session.CreateConversationItems(context.Background(), RealtimeConversationItem{Role: "user", Text: "q", Timestamp: 1}, RealtimeConversationItem{Role: "assistant", Text: "a", Timestamp: 2}); err != nil {
		t.Fatalf("CreateConversationItems error = %v", err)
	}
	if err := session.UpdateConversationItems(context.Background(), RealtimeConversationItem{ItemID: "item-1", Text: "updated"}); err != nil {
		t.Fatalf("UpdateConversationItems error = %v", err)
	}
	if err := session.RetrieveConversationItems(context.Background()); err != nil {
		t.Fatalf("RetrieveConversationItems latest error = %v", err)
	}
	if err := session.RetrieveConversationItems(context.Background(), "item-1"); err != nil {
		t.Fatalf("RetrieveConversationItems error = %v", err)
	}
	if err := session.TruncateConversationItem(context.Background(), "item-1", 1200); err != nil {
		t.Fatalf("TruncateConversationItem error = %v", err)
	}
	if err := session.DeleteConversationItems(context.Background(), "item-1"); err != nil {
		t.Fatalf("DeleteConversationItems error = %v", err)
	}

	writes := conn.writesSnapshot()
	wantFrames := []struct {
		event   int32
		payload string
	}{
		{realtimeUpdateConfigEvent, `{"tts":{"speaker":"zh_female_vv_jupiter_bigtts","audio_config":{"speech_rate":5,"loudness_rate":6}},"dialog":{"dialog_id":"dialog-1","location":{"city":"上海"}}}`},
		{realtimeRAGTextEvent, `{"external_rag":"[{\"title\":\"t\",\"content\":\"c\"}]"}`},
		{realtimeConversationCreate, `{"items":[{"role":"user","text":"q","timestamp":1},{"role":"assistant","text":"a","timestamp":2}]}`},
		{realtimeConversationUpdate, `{"items":[{"item_id":"item-1","text":"updated"}]}`},
		{realtimeConversationRetrieve, `{}`},
		{realtimeConversationRetrieve, `{"items":[{"item_id":"item-1"}]}`},
		{realtimeConversationTruncate, `{"item_id":"item-1","audio_end_ms":1200}`},
		{realtimeConversationDelete, `{"items":[{"item_id":"item-1"}]}`},
	}
	if len(writes) < len(wantFrames)+2 {
		t.Fatalf("writes count = %d, want >= %d", len(writes), len(wantFrames)+2)
	}
	for i, want := range wantFrames {
		frame, err := protocol.ParseServerFrame(writes[len(writes)-len(wantFrames)+i])
		if err != nil {
			t.Fatalf("parse frame %d: %v", i, err)
		}
		if frame.Event != want.event {
			t.Fatalf("frame %d event = %d, want %d", i, frame.Event, want.event)
		}
		if frame.SessionID != session.SessionID() {
			t.Fatalf("frame %d binary session ID = %q, want %q", i, frame.SessionID, session.SessionID())
		}
		if frame.ConnectID != "" {
			t.Fatalf("frame %d binary connect ID = %q, want empty", i, frame.ConnectID)
		}
		assertJSONContract(t, frame.Payload, want.payload)
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
		"dialog_id":"dialog-1",
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
	if evt.DialogID != "dialog-1" {
		t.Fatalf("dialog_id = %q, want dialog-1", evt.DialogID)
	}
	if evt.Usage == nil || evt.Usage.InputTextTokens != 1 || evt.Usage.OutputTextTokens != 2 {
		t.Fatalf("usage = %+v", evt.Usage)
	}
	if len(evt.Results) != 1 || evt.Results[0].Text != "recognized" || evt.Results[0].IsInterim {
		t.Fatalf("results = %+v", evt.Results)
	}
}

func TestRealtimeServerEventWireContract(t *testing.T) {
	tests := []struct {
		name    string
		event   RealtimeEventType
		payload string
		check   func(*testing.T, *RealtimeEvent)
	}{
		{
			name:    "usage",
			event:   EventUsageResponse,
			payload: `{"usage":{"input_text_tokens":1,"input_audio_tokens":2,"cached_text_tokens":3,"cached_audio_tokens":4,"output_text_tokens":5,"output_audio_tokens":6}}`,
			check: func(t *testing.T, evt *RealtimeEvent) {
				t.Helper()
				want := &RealtimeUsage{1, 2, 3, 4, 5, 6}
				if !reflect.DeepEqual(evt.Usage, want) {
					t.Fatalf("usage = %+v, want %+v", evt.Usage, want)
				}
			},
		},
		{
			name:    "tts started",
			event:   EventTTSStarted,
			payload: `{"text":"answer","tts_type":"default","question_id":"q-1","reply_id":"r-1"}`,
			check: func(t *testing.T, evt *RealtimeEvent) {
				t.Helper()
				if evt.Text != "answer" || evt.TTSType != "default" || evt.QuestionID != "q-1" || evt.ReplyID != "r-1" {
					t.Fatalf("TTS event = %+v", evt)
				}
			},
		},
		{
			name:    "asr response",
			event:   EventASRResponse,
			payload: `{"results":[{"text":"partial","is_interim":true},{"text":"final","is_interim":false}]}`,
			check: func(t *testing.T, evt *RealtimeEvent) {
				t.Helper()
				want := []RealtimeASRResult{{Text: "partial", IsInterim: true}, {Text: "final", IsInterim: false}}
				if !reflect.DeepEqual(evt.Results, want) || evt.Text != "partial" || !evt.IsFinal {
					t.Fatalf("ASR event = %+v, want results=%+v and final", evt, want)
				}
			},
		},
		{
			name:    "chat response",
			event:   EventChatResponse,
			payload: `{"content":"reply","question_id":"q-2","reply_id":"r-2"}`,
			check: func(t *testing.T, evt *RealtimeEvent) {
				t.Helper()
				if evt.Text != "reply" || evt.QuestionID != "q-2" || evt.ReplyID != "r-2" {
					t.Fatalf("chat event = %+v", evt)
				}
			},
		},
		{
			name:    "conversation items",
			event:   EventConversationRetrieved,
			payload: `{"items":[{"item_id":"item-1","role":"user","text":"question","timestamp":123}]}`,
			check: func(t *testing.T, evt *RealtimeEvent) {
				t.Helper()
				want := []RealtimeConversationItem{{ItemID: "item-1", Role: "user", Text: "question", Timestamp: 123}}
				if !reflect.DeepEqual(evt.Items, want) {
					t.Fatalf("items = %+v, want %+v", evt.Items, want)
				}
			},
		},
		{
			name:    "tts finished",
			event:   EventTTSFinished,
			payload: `{"status_code":20000000,"question_id":"q-3","reply_id":"r-3"}`,
			check: func(t *testing.T, evt *RealtimeEvent) {
				t.Helper()
				if evt.StatusCode != "20000000" || evt.QuestionID != "q-3" || evt.ReplyID != "r-3" {
					t.Fatalf("TTS finished event = %+v", evt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, conn := newOpenedRealtimeSessionForTest(t, nil)
			defer session.Close()
			conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(tt.event), session.SessionID(), "", []byte(tt.payload)))

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			evt, err := session.RecvEvent(ctx)
			if err != nil {
				t.Fatalf("RecvEvent error = %v", err)
			}
			if evt.Type != tt.event || evt.SessionID != session.SessionID() {
				t.Fatalf("event envelope = %+v, want type=%d session=%q", evt, tt.event, session.SessionID())
			}
			tt.check(t, evt)
		})
	}

	t.Run("binary audio", func(t *testing.T) {
		session, conn := newOpenedRealtimeSessionForTest(t, nil)
		defer session.Close()
		wantAudio := []byte{0x00, 0x01, 0xfe, 0xff}
		conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerAudioFrame(t, session.SessionID(), wantAudio))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		evt, err := session.RecvEvent(ctx)
		if err != nil {
			t.Fatalf("RecvEvent error = %v", err)
		}
		if evt.Type != EventTTSAudioData || !reflect.DeepEqual(evt.Audio, wantAudio) {
			t.Fatalf("audio event = %+v, want bytes %v", evt, wantAudio)
		}
	})
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
		Model: RealtimeModelO20,
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

func TestRealtimeInstructionMappings(t *testing.T) {
	tests := []struct {
		name       string
		model      RealtimeModelVersion
		wireField  string
		unexpected string
	}{
		{name: "O20", model: RealtimeModelVersion(" O20 "), wireField: "system_role", unexpected: "character_manifest"},
		{name: "SC20", model: RealtimeModelVersion(" sc20 "), wireField: "character_manifest", unexpected: "system_role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultRealtimeConfig()
			cfg.TTS.Speaker = realtimeTestSpeaker
			cfg.Model = tt.model
			cfg.Instructions = "只回答收到"

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
			if got := dialog[tt.wireField]; got != cfg.Instructions {
				t.Fatalf("dialog.%s = %v, want instructions", tt.wireField, got)
			}
			if _, ok := dialog[tt.unexpected]; ok {
				t.Fatalf("dialog contains opposite-family field %q: %s", tt.unexpected, payloadBytes)
			}
			if _, ok := payload["instructions"]; ok {
				t.Fatalf("payload contains literal instructions: %s", payloadBytes)
			}
			if _, ok := payload["prompt"]; ok {
				t.Fatalf("canonical instructions populated prompt: %s", payloadBytes)
			}
		})
	}
}

func TestRealtimeInstructionAndCapabilityValidation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*RealtimeConfig)
		want string
	}{
		{name: "missing model", edit: func(cfg *RealtimeConfig) { cfg.Model = "" }, want: "model is required"},
		{name: "resource id", edit: func(cfg *RealtimeConfig) { cfg.ResourceID = "ignored" }, want: "WithResourceID"},
		{name: "O opposite field", edit: func(cfg *RealtimeConfig) { cfg.Dialog.CharacterManifest = "wrong" }, want: "character_manifest"},
		{name: "O conflict", edit: func(cfg *RealtimeConfig) { cfg.Instructions = "a"; cfg.Dialog.SystemRole = "b" }, want: "conflict"},
		{name: "SC opposite fields", edit: func(cfg *RealtimeConfig) { cfg.Model = RealtimeModelSC20; cfg.Dialog.BotName = "wrong" }, want: "O20 fields"},
		{name: "SC conflict", edit: func(cfg *RealtimeConfig) {
			cfg.Model = RealtimeModelSC20
			cfg.Instructions = "a"
			cfg.Dialog.CharacterManifest = "b"
		}, want: "conflict"},
		{name: "SC music", edit: func(cfg *RealtimeConfig) {
			cfg.Model = RealtimeModelSC20
			cfg.Dialog.Extra = &RealtimeDialogExtra{EnableMusic: new(true)}
		}, want: "enable_music"},
		{name: "SC TTS model", edit: func(cfg *RealtimeConfig) {
			cfg.Model = RealtimeModelSC20
			cfg.TTS.Extra = &RealtimeTTSExtra{TTS20Model: "expressive"}
		}, want: "tts_2.0_model"},
		{name: "dialect", edit: func(cfg *RealtimeConfig) { cfg.TTS.Extra = &RealtimeTTSExtra{ExplicitDialect: "guangdong"} }, want: "explicit_dialect"},
		{name: "duplex web type", edit: func(cfg *RealtimeConfig) {
			cfg.Dialog.Extra = &RealtimeDialogExtra{VolcWebsearchType: "web_global_api"}
		}, want: "volc_websearch_type"},
		{name: "web api key", edit: func(cfg *RealtimeConfig) { cfg.Dialog.Extra = &RealtimeDialogExtra{EnableVolcWebsearch: new(true)} }, want: "volc_websearch_api_key"},
		{name: "web agent bot", edit: func(cfg *RealtimeConfig) {
			cfg.Dialog.Extra = &RealtimeDialogExtra{EnableVolcWebsearch: new(true), VolcWebsearchType: "web_agent", VolcWebsearchAPIKey: "secret"}
		}, want: "volc_websearch_bot_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultRealtimeConfig()
			cfg.TTS.Speaker = realtimeTestSpeaker
			cfg.Model = RealtimeModelO20
			tt.edit(&cfg)
			_, err := normalizeRealtimeConfig(&cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("normalizeRealtimeConfig error = %v, want containing %q", err, tt.want)
			}
			apiErr, ok := AsError(err)
			if !ok || apiErr.Code != CodeParamError {
				t.Fatalf("error = %T %v, want CodeParamError", err, err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked field value: %v", err)
			}
		})
	}
}

func TestRealtimeControlPayloadsUseBinarySessionEnvelopeOnly(t *testing.T) {
	cfg := DefaultRealtimeConfig()
	cfg.InputMode = RealtimeInputModePushToTalk
	session, conn := newOpenedRealtimeSessionForTest(t, &cfg)
	defer session.Close()

	for _, send := range []func(context.Context) error{session.EndASR, session.Interrupt, session.FinishSession} {
		if err := send(context.Background()); err != nil {
			t.Fatalf("control send error = %v", err)
		}
	}
	writes := conn.writesSnapshot()
	for _, raw := range writes[len(writes)-3:] {
		frame, err := protocol.ParseServerFrame(raw)
		if err != nil {
			t.Fatalf("parse control frame: %v", err)
		}
		if frame.SessionID != session.SessionID() {
			t.Fatalf("binary session id = %q, want %q", frame.SessionID, session.SessionID())
		}
		if string(frame.Payload) != "{}" {
			t.Fatalf("control payload = %s, want {}", frame.Payload)
		}
	}
}

func TestRealtimeUpdateConfigProjectsExactFieldsAndRejectsBeforeWrite(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	err := session.UpdateConfig(context.Background(), RealtimeUpdateConfig{
		TTS: &RealtimeTTSConfig{Speaker: " voice ", AudioConfig: RealtimeAudioConfig{SpeechRate: 3, LoudnessRate: -2}},
		Dialog: &RealtimeDialogConfig{
			DialogID:      "dialog-2",
			BotName:       "bot",
			SystemRole:    "role",
			SpeakingStyle: "style",
			Location:      &RealtimeLocation{City: "北京"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig error = %v", err)
	}
	writes := conn.writesSnapshot()
	frame, err := protocol.ParseServerFrame(writes[len(writes)-1])
	if err != nil {
		t.Fatalf("parse update frame: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		t.Fatalf("unmarshal update payload: %v", err)
	}
	if _, ok := payload["session_id"]; ok {
		t.Fatalf("update payload leaked session_id: %s", frame.Payload)
	}
	tts := payload["tts"].(map[string]any)
	if got := tts["speaker"]; got != "voice" {
		t.Fatalf("speaker = %v, want trimmed voice", got)
	}
	if len(tts) != 2 {
		t.Fatalf("tts keys = %#v, want speaker and audio_config only", tts)
	}

	before := len(conn.writesSnapshot())
	invalid := []RealtimeUpdateConfig{
		{TTS: &RealtimeTTSConfig{Speaker: "voice", AudioConfig: RealtimeAudioConfig{Format: FormatPCMS16LE}}},
		{Dialog: &RealtimeDialogConfig{CharacterManifest: "unsupported"}},
		{Dialog: &RealtimeDialogConfig{DialogContext: []RealtimeDialogContextItem{{Role: "user", Text: "x"}}}},
	}
	for _, update := range invalid {
		if err := session.UpdateConfig(context.Background(), update); err == nil {
			t.Fatalf("UpdateConfig(%+v) expected error", update)
		}
	}
	if got := len(conn.writesSnapshot()); got != before {
		t.Fatalf("invalid updates wrote %d frames", got-before)
	}

	scCfg := DefaultRealtimeConfig()
	scCfg.Model = RealtimeModelSC20
	scSession, scConn := newOpenedRealtimeSessionForTest(t, &scCfg)
	defer scSession.Close()
	before = len(scConn.writesSnapshot())
	if err := scSession.UpdateConfig(context.Background(), RealtimeUpdateConfig{Dialog: &RealtimeDialogConfig{SystemRole: "no"}}); err == nil {
		t.Fatalf("SC20 O-only update expected error")
	}
	if got := len(scConn.writesSnapshot()); got != before {
		t.Fatalf("invalid SC20 update wrote %d frames", got-before)
	}
}

func TestRealtimeConversationPayloadProjectionAndValidation(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	if err := session.CreateConversationItems(context.Background(),
		RealtimeConversationItem{Role: "user", Text: "q", Timestamp: 1},
		RealtimeConversationItem{Role: "assistant", Text: "a", Timestamp: 2},
	); err != nil {
		t.Fatalf("CreateConversationItems error = %v", err)
	}
	frame, err := protocol.ParseServerFrame(conn.writesSnapshot()[len(conn.writesSnapshot())-1])
	if err != nil {
		t.Fatalf("parse create frame: %v", err)
	}
	var createPayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(frame.Payload, &createPayload); err != nil {
		t.Fatalf("unmarshal create payload: %v", err)
	}
	if len(createPayload.Items) != 2 || len(createPayload.Items[0]) != 3 {
		t.Fatalf("create items = %#v, want exact role/text/timestamp", createPayload.Items)
	}

	if err := session.UpdateConversationItems(context.Background(), RealtimeConversationItem{ItemID: "item-1", Text: "new"}); err != nil {
		t.Fatalf("UpdateConversationItems error = %v", err)
	}
	writes := conn.writesSnapshot()
	frame, err = protocol.ParseServerFrame(writes[len(writes)-1])
	if err != nil {
		t.Fatalf("parse update frame: %v", err)
	}
	var updatePayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(frame.Payload, &updatePayload); err != nil {
		t.Fatalf("unmarshal update payload: %v", err)
	}
	if len(updatePayload.Items) != 1 || len(updatePayload.Items[0]) != 2 {
		t.Fatalf("update items = %#v, want exact item_id/text", updatePayload.Items)
	}

	before := len(conn.writesSnapshot())
	invalidCalls := []func() error{
		func() error {
			return session.CreateConversationItems(context.Background(), RealtimeConversationItem{Role: "user", Text: "odd"})
		},
		func() error {
			return session.CreateConversationItems(context.Background(), RealtimeConversationItem{ItemID: "leak", Role: "user", Text: "q"}, RealtimeConversationItem{Role: "assistant", Text: "a"})
		},
		func() error {
			return session.CreateConversationItems(context.Background(), RealtimeConversationItem{Role: "user", Text: "q", Timestamp: time.Now().Add(time.Hour).Unix()}, RealtimeConversationItem{Role: "assistant", Text: "a", Timestamp: time.Now().Add(2 * time.Hour).Unix()})
		},
		func() error {
			return session.UpdateConversationItems(context.Background(), RealtimeConversationItem{ItemID: "item", Role: "user", Text: "leak"})
		},
	}
	for _, call := range invalidCalls {
		if err := call(); err == nil {
			t.Fatalf("invalid conversation operation expected error")
		}
	}
	if got := len(conn.writesSnapshot()); got != before {
		t.Fatalf("invalid conversation operations wrote %d frames", got-before)
	}
}

func TestRealtimeSessionExposesNegotiatedDialogID(t *testing.T) {
	session, _ := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()
	if session.DialogID() != "dialog-from-server" {
		t.Fatalf("DialogID = %q, want dialog-from-server", session.DialogID())
	}
	if session.DialogID() == session.SessionID() {
		t.Fatalf("dialog ID must remain distinct from session ID")
	}
}

func TestRealtimeOperationErrorsAreDecodedAndNonfatal(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventConversationDeleted), session.SessionID(), "", []byte(`{"status_code":40000010,"message":"empty conversation deleted messages"}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventDialogCommonError), session.SessionID(), "", []byte(`{"status_code":"MODEL_BUSY","message":"try again"}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventChatResponse), session.SessionID(), "", []byte(`{"content":"still alive"}`)))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	deleted, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("deleted RecvEvent error = %v", err)
	}
	if deleted.Error == nil || deleted.Error.Code != 40000010 || deleted.Message == "" {
		t.Fatalf("deleted event = %+v, want mapped provider error", deleted)
	}
	if deleted.Error.ReqID != "" {
		t.Fatalf("deleted error reqid = %q, must not reuse session ID", deleted.Error.ReqID)
	}
	common, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("common RecvEvent error = %v", err)
	}
	if common.Error == nil || common.Error.Code != CodeServerError || common.StatusCode != "MODEL_BUSY" {
		t.Fatalf("common event = %+v, want nonnumeric status fallback", common)
	}
	alive, err := session.RecvEvent(ctx)
	if err != nil {
		t.Fatalf("alive RecvEvent error = %v", err)
	}
	if alive.Text != "still alive" {
		t.Fatalf("alive event = %+v, operation error terminated receive loop", alive)
	}
}

func TestRealtimeHandshakeFailureReturnsProviderMessage(t *testing.T) {
	client := NewClient("test-app", WithAPIKey("key-test"), WithUserID("tester"))
	conn := newFakeWSConn()
	svc := newRealtimeService(client)
	svc.dialer = &fakeDialer{conn: conn}
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventConnectionFailed), "", "connect-1", []byte(`{"error":"provider connection failure"}`)))

	_, err := svc.Dial(context.Background())
	if err == nil || !strings.Contains(err.Error(), "provider connection failure") {
		t.Fatalf("Dial error = %v, want provider message", err)
	}
	apiErr, ok := AsError(err)
	if !ok || apiErr.Code != CodeServerError {
		t.Fatalf("Dial error = %T %v, want CodeServerError", err, err)
	}
}

func TestRealtimeSessionFailureReturnsProviderMessage(t *testing.T) {
	client := NewClient("test-app", WithAPIKey("key-test"), WithUserID("tester"))
	conn := newFakeWSConn()
	svc := newRealtimeService(client)
	svc.dialer = &fakeDialer{conn: conn}
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, protocol.EventConnectionStarted, "", "connect-1", []byte(`{}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventSessionFailed), "session-1", "", []byte(`{"error":"provider session failure"}`)))

	cfg := DefaultRealtimeConfig()
	cfg.TTS.Speaker = realtimeTestSpeaker
	cfg.Model = RealtimeModelO20
	_, err := svc.OpenSession(context.Background(), &cfg)
	if err == nil || !strings.Contains(err.Error(), "provider session failure") {
		t.Fatalf("OpenSession error = %v, want provider message", err)
	}
}

func TestRealtimeLifecycleAndRAGValidationRejectBeforeWrite(t *testing.T) {
	session, conn := newOpenedRealtimeSessionForTest(t, nil)
	defer session.Close()
	before := len(conn.writesSnapshot())

	for _, call := range []func() error{
		func() error { return session.EndASR(context.Background()) },
		func() error { return session.Interrupt(context.Background()) },
		func() error { return session.TruncateConversationItem(context.Background(), "item", 1) },
		func() error { return session.SendRAGText(context.Background(), strings.Repeat("界", 4001)) },
	} {
		if err := call(); err == nil {
			t.Fatalf("lifecycle/RAG validation expected error")
		}
	}
	if got := len(conn.writesSnapshot()); got != before {
		t.Fatalf("invalid lifecycle/RAG calls wrote %d frames", got-before)
	}
}

func newOpenedRealtimeSessionForTest(t *testing.T, cfg *RealtimeConfig) (*RealtimeSession, *fakeWSConn) {
	t.Helper()

	client := NewClient("test-app", WithAPIKey("key-test"), WithUserID("tester"))
	conn := newFakeWSConn()
	dialer := &fakeDialer{conn: conn}

	svc := newRealtimeService(client)
	svc.dialer = dialer

	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, protocol.EventConnectionStarted, "", "connect-1", []byte(`{"ok":true}`)))
	conn.enqueue(websocket.BinaryMessage, mustBuildRealtimeServerEventFrame(t, int32(EventSessionStarted), "session-1", "", []byte(`{"dialog_id":"dialog-from-server"}`)))

	if cfg == nil {
		defaultConfig := DefaultRealtimeConfig()
		cfg = &defaultConfig
	} else {
		configCopy := *cfg
		cfg = &configCopy
	}
	if cfg.Model == "" {
		cfg.Model = RealtimeModelO20
	}
	if cfg.TTS.Speaker == "" {
		cfg.TTS.Speaker = realtimeTestSpeaker
	}

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

func mustBuildRealtimeServerAudioFrame(t *testing.T, sessionID string, payload []byte) []byte {
	t.Helper()

	raw, err := protocol.BuildEventFrame(protocol.EventFrame{
		MessageType:   protocol.MessageTypeAudioOnlyServer,
		Flags:         protocol.FlagWithEvent,
		Event:         int32(EventTTSAudioData),
		SessionID:     sessionID,
		Serialization: protocol.SerializationNone,
		Compression:   protocol.CompressionNone,
		Payload:       payload,
	})
	if err != nil {
		t.Fatalf("BuildEventFrame audio error = %v", err)
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

func assertJSONContract(t *testing.T, got []byte, want string) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal actual JSON: %v; payload=%s", err, got)
	}
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("unmarshal expected JSON: %v; payload=%s", err, want)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON contract mismatch\n got: %s\nwant: %s", got, want)
	}
}

func assertNoAnyMapFields(t *testing.T, typ reflect.Type, path string, seen map[reflect.Type]bool) {
	t.Helper()

	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	if seen[typ] {
		return
	}
	seen[typ] = true

	anyType := reflect.TypeFor[any]()
	for field := range typ.Fields() {
		fieldPath := path + "." + field.Name
		fieldType := field.Type
		if fieldType.Kind() == reflect.Map && fieldType.Key().Kind() == reflect.String && fieldType.Elem() == anyType {
			t.Fatalf("%s exposes map[string]any passthrough", fieldPath)
		}
		assertNoAnyMapFields(t, fieldType, fieldPath, seen)
	}
}
