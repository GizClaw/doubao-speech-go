package doubaospeech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/GizClaw/doubao-speech-go/internal/transport"
	"github.com/GizClaw/doubao-speech-go/internal/util"
	"github.com/gorilla/websocket"
)

const (
	podcastEndpointPath               = "/api/v3/sami/podcasttts"
	podcastDefaultAppKey              = "aGjiRDfUWi"
	podcastStartSessionEvent    int32 = 100
	podcastFinishSessionEvent   int32 = 102
	podcastSessionStartedEvent  int32 = 150
	podcastSessionFinishedEvent int32 = 152
	podcastSessionFailedEvent   int32 = 153
	podcastCloseTimeout               = 2 * time.Second
)

// PodcastService provides streamed podcast generation.
type PodcastService struct {
	client *Client
	dialer transport.WSDialer
}

func newPodcastService(c *Client) *PodcastService {
	return &PodcastService{client: c, dialer: transport.NewGorillaDialer(nil)}
}

// OpenSession connects, starts generation, and returns a stream of podcast events.
func (s *PodcastService) OpenSession(ctx context.Context, request *PodcastRequest) (*PodcastSession, error) {
	if s == nil || s.client == nil {
		return nil, newAPIError(CodeParamError, "podcast service is not initialized")
	}
	normalized, err := normalizePodcastRequest(request)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.client.config.apiKey) == "" && strings.TrimSpace(s.client.config.accessKey) == "" {
		return nil, newAPIError(CodeAuthError, "podcast API key and access key are both empty")
	}

	connectID := util.NewReqID("podcast")
	headers := http.Header{}
	if s.client.config.apiKey != "" {
		headers.Set("X-Api-Key", s.client.config.apiKey)
	}
	if s.client.config.accessKey != "" {
		headers.Set("X-Api-App-Id", s.client.config.appID)
		headers.Set("X-Api-App-Key", firstNonEmpty(s.client.config.appKey, podcastDefaultAppKey))
		headers.Set("X-Api-Access-Key", s.client.config.accessKey)
	}
	headers.Set("X-Api-Resource-Id", s.client.resolveResourceID("", ResourcePodcast))
	headers.Set("X-Api-Connect-Id", connectID)

	endpoint := strings.TrimRight(s.client.config.wsURL, "/") + podcastEndpointPath
	conn, resp, err := s.dialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		return nil, wsConnectError(err, resp, responseMetadata{ReqID: normalized.SessionID, ConnectID: connectID})
	}
	session := &PodcastSession{conn: conn, sessionID: normalized.SessionID, connectID: connectID}
	if err := session.writeEvent(ctx, protocol.EventStartConnection, "", []byte("{}")); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "send podcast start connection")
	}
	if _, err := session.expectEvent(ctx, protocol.EventConnectionStarted); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "read podcast connection response")
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		_ = session.Close()
		return nil, wrapError(err, "marshal podcast request")
	}
	if err := session.writeEvent(ctx, podcastStartSessionEvent, normalized.SessionID, payload); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "send podcast start session")
	}
	started, err := session.expectEvent(ctx, podcastSessionStartedEvent)
	if err != nil {
		_ = session.Close()
		return nil, wrapError(err, "read podcast start response")
	}
	session.taskID = podcastTaskID(started.Payload, normalized.SessionID)
	if err := session.writeEvent(ctx, podcastFinishSessionEvent, normalized.SessionID, []byte("{}")); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "send podcast finish input")
	}
	return session, nil
}

// PodcastSession streams events for one podcast task.
type PodcastSession struct {
	conn      transport.WSConn
	sessionID string
	taskID    string
	connectID string
	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// TaskID returns the identifier to use in PodcastRetryInfo after interruption.
func (s *PodcastSession) TaskID() string { return s.taskID }

// RecvEvent reads the next podcast event. PodcastEventSessionFinished marks a
// successful terminal event.
func (s *PodcastSession) RecvEvent(ctx context.Context) (*PodcastEvent, error) {
	frame, err := s.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	if frame.MessageType == protocol.MessageTypeError {
		return nil, withErrorMetadata(parseWSErrorPayload(frame.Payload, frame.ErrorCode), responseMetadata{ReqID: s.sessionID, ConnectID: s.connectID})
	}
	if !frame.HasEvent {
		return nil, fmt.Errorf("podcast response is missing an event")
	}
	if frame.Event == podcastSessionFailedEvent {
		return nil, withErrorMetadata(parseWSErrorPayload(frame.Payload, 0), responseMetadata{ReqID: s.sessionID, ConnectID: s.connectID})
	}
	event := &PodcastEvent{Type: PodcastEventType(frame.Event), SessionID: frame.SessionID, Payload: copyBytes(frame.Payload)}
	switch event.Type {
	case PodcastEventRoundStarted:
		if err := json.Unmarshal(frame.Payload, event); err != nil {
			return nil, wrapError(err, "decode podcast round")
		}
		event.Type = PodcastEventRoundStarted
		event.SessionID = frame.SessionID
		event.Payload = copyBytes(frame.Payload)
	case PodcastEventAudio:
		event.Audio = copyBytes(frame.Payload)
	case PodcastEventRoundFinished:
		if len(frame.Payload) > 0 {
			if err := json.Unmarshal(frame.Payload, event); err != nil {
				return nil, wrapError(err, "decode podcast round completion")
			}
		}
		event.Type = PodcastEventRoundFinished
		event.SessionID = frame.SessionID
		event.Payload = copyBytes(frame.Payload)
	case PodcastEventSessionFinished:
		return event, nil
	default:
		return event, nil
	}
	return event, nil
}

// Close finishes the connection and releases the websocket.
func (s *PodcastSession) Close() error {
	s.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), podcastCloseTimeout)
		defer cancel()
		finishErr := s.writeEvent(ctx, protocol.EventFinishConnection, "", []byte("{}"))
		s.closeErr = errors.Join(finishErr, s.conn.Close())
	})
	return s.closeErr
}

func (s *PodcastSession) expectEvent(ctx context.Context, expected int32) (*protocol.ParsedFrame, error) {
	frame, err := s.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	if frame.MessageType == protocol.MessageTypeError {
		return nil, withErrorMetadata(parseWSErrorPayload(frame.Payload, frame.ErrorCode), responseMetadata{ReqID: s.sessionID, ConnectID: s.connectID})
	}
	if !frame.HasEvent || frame.Event != expected {
		return nil, fmt.Errorf("unexpected podcast response event: %d", frame.Event)
	}
	return frame, nil
}

func (s *PodcastSession) readFrame(ctx context.Context) (*protocol.ParsedFrame, error) {
	messageType, payload, err := readWSMessageWithContext(ctx, s.conn)
	if err != nil {
		return nil, err
	}
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected podcast websocket message type: %d", messageType)
	}
	return protocol.ParseServerFrame(payload)
}

func (s *PodcastSession) writeEvent(ctx context.Context, event int32, sessionID string, payload []byte) error {
	packet, err := protocol.BuildFullClientJSONWithEvent(event, sessionID, payload)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeWSMessageWithContext(ctx, s.conn, websocket.BinaryMessage, packet)
}

func writeWSMessageWithContext(ctx context.Context, conn transport.WSConn, messageType int, payload []byte) error {
	if ctx == nil {
		return conn.WriteMessage(messageType, payload)
	}
	result := make(chan error, 1)
	go func() { result <- conn.WriteMessage(messageType, payload) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		_ = conn.Close()
		return ctx.Err()
	}
}

func podcastTaskID(payload []byte, fallback string) string {
	var response struct {
		TaskID    string `json:"task_id"`
		SessionID string `json:"session_id"`
		Data      *struct {
			TaskID    string `json:"task_id"`
			SessionID string `json:"session_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return fallback
	}
	values := []string{response.TaskID, response.SessionID}
	if response.Data != nil {
		values = append(values, response.Data.TaskID, response.Data.SessionID)
	}
	return firstNonEmpty(append(values, fallback)...)
}

func normalizePodcastRequest(request *PodcastRequest) (PodcastRequest, error) {
	if request == nil {
		return PodcastRequest{}, newAPIError(CodeParamError, "podcast request is nil")
	}
	normalized := *request
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	if normalized.SessionID == "" {
		normalized.SessionID = util.NewReqID("podcast-session")
	}
	normalized.InputID = strings.TrimSpace(normalized.InputID)
	normalized.PromptText = strings.TrimSpace(normalized.PromptText)
	normalized.InputText = strings.TrimSpace(normalized.InputText)
	if normalized.InputInfo != nil {
		inputInfo := *normalized.InputInfo
		inputInfo.InputURL = strings.TrimSpace(inputInfo.InputURL)
		normalized.InputInfo = &inputInfo
	}
	if len(normalized.NLPTexts) > 0 {
		normalized.NLPTexts = append([]PodcastScriptLine(nil), normalized.NLPTexts...)
		for index := range normalized.NLPTexts {
			normalized.NLPTexts[index].Speaker = PodcastSpeakerID(strings.TrimSpace(string(normalized.NLPTexts[index].Speaker)))
			normalized.NLPTexts[index].Text = strings.TrimSpace(normalized.NLPTexts[index].Text)
			if normalized.NLPTexts[index].Speaker == "" || normalized.NLPTexts[index].Text == "" {
				return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("podcast nlp_texts[%d] requires speaker and text", index))
			}
		}
	}
	if normalized.InputID == "" {
		return PodcastRequest{}, newAPIError(CodeParamError, "podcast input_id is empty")
	}
	inputURL := ""
	if normalized.InputInfo != nil {
		inputURL = normalized.InputInfo.InputURL
	}
	if normalized.PromptText == "" && normalized.InputText == "" && inputURL == "" && len(normalized.NLPTexts) == 0 {
		return PodcastRequest{}, newAPIError(CodeParamError, "podcast prompt_text, input_text, input_url, and nlp_texts are all empty")
	}
	if normalized.Action == nil {
		action := PodcastActionFromPrompt
		if len(normalized.NLPTexts) > 0 {
			action = PodcastActionFromScript
		} else if normalized.InputText != "" || inputURL != "" {
			action = PodcastActionFromSource
		}
		normalized.Action = &action
	}
	switch *normalized.Action {
	case PodcastActionFromSource:
		if normalized.InputText == "" && inputURL == "" {
			return PodcastRequest{}, newAPIError(CodeParamError, "podcast source action requires input_text or input_url")
		}
	case PodcastActionFromScript:
		if len(normalized.NLPTexts) == 0 {
			return PodcastRequest{}, newAPIError(CodeParamError, "podcast script action requires nlp_texts")
		}
	case PodcastActionFromPrompt:
		if normalized.PromptText == "" {
			return PodcastRequest{}, newAPIError(CodeParamError, "podcast prompt action requires prompt_text")
		}
	default:
		return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("unsupported podcast action: %d", *normalized.Action))
	}
	if normalized.InputInfo == nil {
		normalized.InputInfo = &PodcastInputInfo{}
	}
	if normalized.AudioConfig == nil {
		normalized.AudioConfig = &PodcastAudioConfig{Format: FormatMP3, SampleRate: SampleRate24000}
	}
	if normalized.SpeakerInfo != nil {
		speakerInfo := *normalized.SpeakerInfo
		speakerInfo.Speakers = append([]PodcastSpeakerID(nil), normalized.SpeakerInfo.Speakers...)
		speakerInfo.SpeakerAdditions = append(PodcastSpeakerAdditions(nil), normalized.SpeakerInfo.SpeakerAdditions...)
		if len(speakerInfo.Speakers) != 0 && len(speakerInfo.Speakers) != 2 {
			return PodcastRequest{}, newAPIError(CodeParamError, "podcast speaker_info.speakers must contain exactly two speakers")
		}
		knownSpeakers := make([]PodcastSpeakerID, 0, len(speakerInfo.Speakers))
		for index := range speakerInfo.Speakers {
			speakerInfo.Speakers[index] = PodcastSpeakerID(strings.TrimSpace(string(speakerInfo.Speakers[index])))
			if speakerInfo.Speakers[index] == "" {
				return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("podcast speaker_info.speakers[%d] is empty", index))
			}
			if podcastSpeakerSliceContains(knownSpeakers, speakerInfo.Speakers[index]) {
				return PodcastRequest{}, newAPIError(CodeParamError, "podcast speaker_info.speakers contains a duplicate speaker")
			}
			knownSpeakers = append(knownSpeakers, speakerInfo.Speakers[index])
		}
		additionSpeakers := make([]PodcastSpeakerID, 0, len(speakerInfo.SpeakerAdditions))
		for index := range speakerInfo.SpeakerAdditions {
			addition := &speakerInfo.SpeakerAdditions[index]
			addition.Speaker = PodcastSpeakerID(strings.TrimSpace(string(addition.Speaker)))
			addition.Model = PodcastSpeakerModel(strings.TrimSpace(string(addition.Model)))
			if addition.Speaker == "" {
				return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("podcast speaker_info.speaker_additions[%d].speaker is empty", index))
			}
			if addition.Model == "" {
				return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("podcast speaker_info.speaker_additions[%d].model is empty", index))
			}
			if !podcastSpeakerSliceContains(knownSpeakers, addition.Speaker) {
				return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("podcast speaker addition %q is not present in speakers", addition.Speaker))
			}
			if podcastSpeakerSliceContains(additionSpeakers, addition.Speaker) {
				return PodcastRequest{}, newAPIError(CodeParamError, fmt.Sprintf("podcast speaker addition %q is duplicated", addition.Speaker))
			}
			additionSpeakers = append(additionSpeakers, addition.Speaker)
		}
		normalized.SpeakerInfo = &speakerInfo
	}
	return normalized, nil
}

func podcastSpeakerSliceContains(values []PodcastSpeakerID, target PodcastSpeakerID) bool {
	return slices.Contains(values, target)
}
