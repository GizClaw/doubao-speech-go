package doubaospeech

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
)

const (
	RealtimeDuplexEventSessionCreate                     = "session.create"
	RealtimeDuplexEventSessionUpdate                     = "session.update"
	RealtimeDuplexEventSessionClose                      = "session.close"
	RealtimeDuplexEventInputAudioBufferAppend            = "input_audio_buffer.append"
	RealtimeDuplexEventInputAudioBufferCommit            = "input_audio_buffer.commit"
	RealtimeDuplexEventSpeechTextBufferAppend            = "speech_text_buffer.append"
	RealtimeDuplexEventSpeechTextBufferCommit            = "speech_text_buffer.commit"
	RealtimeDuplexEventSpeechTextBufferReplacementAppend = "speech_text_buffer.replacement.append"
	RealtimeDuplexEventSpeechTextBufferReplacementCommit = "speech_text_buffer.replacement.commit"
	RealtimeDuplexEventConversationItemCreate            = "conversation.item.create"
	RealtimeDuplexEventConversationItemUpdate            = "conversation.item.update"
	RealtimeDuplexEventConversationItemRetrieve          = "conversation.item.retrieve"
	RealtimeDuplexEventConversationItemDelete            = "conversation.item.delete"
	RealtimeDuplexEventResponseCancel                    = "response.cancel"

	RealtimeDuplexEventSessionCreated = "session.created"
	RealtimeDuplexEventSessionUpdated = "session.updated"
	RealtimeDuplexEventSessionClosed  = "session.closed"

	RealtimeDuplexEventInputAudioBufferCommitted = "input_audio_buffer.committed"

	RealtimeDuplexEventTranscriptionStarted   = "conversation.item.input_audio_transcription.started"
	RealtimeDuplexEventTranscriptionDelta     = "conversation.item.input_audio_transcription.delta"
	RealtimeDuplexEventTranscriptionCompleted = "conversation.item.input_audio_transcription.completed"
	RealtimeDuplexEventTranscriptionFailed    = "conversation.item.input_audio_transcription.failed"

	RealtimeDuplexEventResponseOutputTextDelta = "response.output_text.delta"
	RealtimeDuplexEventResponseOutputTextDone  = "response.output_text.done"

	RealtimeDuplexEventResponseOutputAudioStarted = "response.output_audio.started"
	RealtimeDuplexEventResponseOutputAudioDelta   = "response.output_audio.delta"
	RealtimeDuplexEventResponseOutputAudioDone    = "response.output_audio.done"

	RealtimeDuplexEventConversationItemAdded     = "conversation.item.added"
	RealtimeDuplexEventConversationItemRetrieved = "conversation.item.retrieved"
	RealtimeDuplexEventConversationItemUpdated   = "conversation.item.updated"
	RealtimeDuplexEventConversationItemDeleted   = "conversation.item.deleted"

	RealtimeDuplexEventResponseFunctionCallArgumentsDone = "response.function_call_arguments.done"
	RealtimeDuplexEventResponseCanceled                  = "response.canceled"
	RealtimeDuplexEventResponseDone                      = "response.done"
	RealtimeDuplexEventError                             = "error"
)

type realtimeDuplexBaseEvent struct {
	Type    string `json:"type"`
	EventID string `json:"event_id,omitempty"`
}

type realtimeDuplexSessionEvent struct {
	Type      string                      `json:"type"`
	EventID   string                      `json:"event_id,omitempty"`
	Session   RealtimeDuplexSessionConfig `json:"session"`
	Extension map[string]any              `json:"extension,omitempty"`
}

type realtimeDuplexSimpleEvent struct {
	Type    string `json:"type"`
	EventID string `json:"event_id,omitempty"`
}

type realtimeDuplexAudioAppendEvent struct {
	Type    string `json:"type"`
	EventID string `json:"event_id,omitempty"`
	Audio   string `json:"audio"`
}

type realtimeDuplexSpeechTextEvent struct {
	Type      string `json:"type"`
	EventID   string `json:"event_id,omitempty"`
	SpeechID  string `json:"speech_id,omitempty"`
	Text      string `json:"text,omitempty"`
	TTSPrompt string `json:"tts_prompt,omitempty"`
}

type realtimeDuplexConversationItemsEvent struct {
	Type    string                           `json:"type"`
	EventID string                           `json:"event_id,omitempty"`
	Items   []RealtimeDuplexConversationItem `json:"items"`
}

// RealtimeDuplexEvent is one parsed server event.
type RealtimeDuplexEvent struct {
	Type    string          `json:"type"`
	EventID string          `json:"event_id,omitempty"`
	Raw     json.RawMessage `json:"-"`

	SessionID  string `json:"session_id,omitempty"`
	ItemID     string `json:"item_id,omitempty"`
	QuestionID string `json:"question_id,omitempty"`
	ResponseID string `json:"response_id,omitempty"`

	Delta      string `json:"delta,omitempty"`
	Text       string `json:"text,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	TTSType    string `json:"tts_type,omitempty"`
	StatusCode string `json:"status_code,omitempty"`

	Audio []byte `json:"-"`

	ConversationItems []RealtimeDuplexConversationItem `json:"items,omitempty"`
	FunctionCalls     []RealtimeDuplexFunctionCall     `json:"function_calls,omitempty"`
	Usage             json.RawMessage                  `json:"usage,omitempty"`
	Error             *Error                           `json:"error,omitempty"`
}

func decodeRealtimeDuplexEvent(payload []byte) (*RealtimeDuplexEvent, error) {
	var base realtimeDuplexBaseEvent
	if err := json.Unmarshal(payload, &base); err != nil {
		return nil, wrapError(err, "decode realtime duplex event type")
	}
	if base.Type == "" {
		return nil, newAPIError(CodeServerError, "realtime duplex event missing type")
	}

	evt := &RealtimeDuplexEvent{
		Type:    base.Type,
		EventID: base.EventID,
		Raw:     copyBytes(payload),
	}

	switch base.Type {
	case RealtimeDuplexEventSessionCreated, RealtimeDuplexEventSessionUpdated:
		var body struct {
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.SessionID = body.Session.ID
	case RealtimeDuplexEventTranscriptionDelta, RealtimeDuplexEventTranscriptionCompleted, RealtimeDuplexEventTranscriptionStarted:
		var body struct {
			ItemID     string `json:"item_id"`
			Delta      string `json:"delta"`
			Transcript string `json:"transcript"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.ItemID = body.ItemID
		evt.Delta = body.Delta
		evt.Transcript = body.Transcript
	case RealtimeDuplexEventResponseOutputTextDelta, RealtimeDuplexEventResponseOutputTextDone:
		var body struct {
			QuestionID string `json:"question_id"`
			ResponseID string `json:"response_id"`
			Delta      string `json:"delta"`
			Text       string `json:"text"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.QuestionID = body.QuestionID
		evt.ResponseID = body.ResponseID
		evt.Delta = body.Delta
		evt.Text = body.Text
	case RealtimeDuplexEventResponseOutputAudioStarted, RealtimeDuplexEventResponseOutputAudioDelta, RealtimeDuplexEventResponseOutputAudioDone:
		var body struct {
			QuestionID string `json:"question_id"`
			ResponseID string `json:"response_id"`
			TTSType    string `json:"tts_type"`
			Delta      string `json:"delta"`
			StatusCode string `json:"status_code"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.QuestionID = body.QuestionID
		evt.ResponseID = body.ResponseID
		evt.TTSType = body.TTSType
		evt.Delta = body.Delta
		evt.StatusCode = body.StatusCode
		if body.Delta != "" {
			audio, err := base64.StdEncoding.DecodeString(body.Delta)
			if err != nil {
				return evt, wrapError(err, "decode realtime duplex audio delta")
			}
			evt.Audio = audio
		}
	case RealtimeDuplexEventConversationItemAdded, RealtimeDuplexEventConversationItemRetrieved, RealtimeDuplexEventConversationItemUpdated, RealtimeDuplexEventConversationItemDeleted:
		var body struct {
			Items []RealtimeDuplexConversationItem `json:"items"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.ConversationItems = body.Items
	case RealtimeDuplexEventResponseFunctionCallArgumentsDone:
		var body struct {
			Items []RealtimeDuplexFunctionCall `json:"items"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.FunctionCalls = body.Items
	case RealtimeDuplexEventResponseDone:
		var body struct {
			Usage json.RawMessage `json:"usage"`
		}
		_ = json.Unmarshal(payload, &body)
		evt.Usage = body.Usage
	case RealtimeDuplexEventError:
		evt.Error = parseRealtimeDuplexError(payload)
		return evt, evt.Error
	}

	return evt, nil
}

func parseRealtimeDuplexError(payload []byte) *Error {
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
			Param   any    `json:"param"`
			EventID string `json:"event_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return newAPIError(CodeServerError, string(payload))
	}

	code := CodeServerError
	if parsed, err := strconv.Atoi(body.Error.Code); err == nil && parsed != 0 {
		code = parsed
	}
	msg := body.Error.Message
	if msg == "" {
		msg = body.Error.Type
	}
	if msg == "" {
		msg = "realtime duplex error"
	}

	return &Error{Code: code, Message: msg, ReqID: body.Error.EventID}
}
