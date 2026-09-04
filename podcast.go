package doubaospeech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	if strings.TrimSpace(s.client.config.accessKey) == "" {
		return nil, newAPIError(CodeAuthError, "podcast access key is empty")
	}
	if strings.TrimSpace(s.client.config.appID) == "" {
		return nil, newAPIError(CodeAuthError, "podcast app id is empty")
	}

	connectID := util.NewReqID("podcast")
	headers := http.Header{}
	headers.Set("X-Api-App-Id", s.client.config.appID)
	headers.Set("X-Api-App-Key", firstNonEmpty(s.client.config.appKey, podcastDefaultAppKey))
	headers.Set("X-Api-Access-Key", s.client.config.accessKey)
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
	if _, err := session.expectEvent(ctx, podcastSessionStartedEvent); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "read podcast start response")
	}
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
	connectID string
	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// TaskID returns the identifier to use in PodcastRetryInfo after interruption.
func (s *PodcastSession) TaskID() string { return s.sessionID }

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
		finishErr := s.writeEvent(context.Background(), protocol.EventFinishConnection, "", []byte("{}"))
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
	return s.conn.WriteMessage(websocket.BinaryMessage, packet)
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
	if normalized.InputID == "" {
		return PodcastRequest{}, newAPIError(CodeParamError, "podcast input_id is empty")
	}
	if normalized.PromptText == "" && normalized.InputText == "" {
		return PodcastRequest{}, newAPIError(CodeParamError, "podcast prompt_text and input_text are both empty")
	}
	if normalized.Action == nil {
		action := 4
		if normalized.InputText != "" {
			action = 0
		}
		normalized.Action = &action
	}
	if normalized.InputInfo == nil {
		normalized.InputInfo = &PodcastInputInfo{}
	}
	if normalized.AudioConfig == nil {
		normalized.AudioConfig = &PodcastAudioConfig{Format: FormatMP3, SampleRate: SampleRate24000}
	}
	return normalized, nil
}
