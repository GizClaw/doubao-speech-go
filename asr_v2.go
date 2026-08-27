package doubaospeech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GizClaw/doubao-speech-go/internal/auth"
	"github.com/GizClaw/doubao-speech-go/internal/protocol"
	"github.com/GizClaw/doubao-speech-go/internal/transport"
	"github.com/GizClaw/doubao-speech-go/internal/util"
	"github.com/gorilla/websocket"
)

// ASRServiceV2 provides SAUC WebSocket streaming recognition.
type ASRServiceV2 struct {
	client *Client
	dialer transport.WSDialer
}

func newASRServiceV2(c *Client) *ASRServiceV2 {
	return &ASRServiceV2{
		client: c,
		dialer: transport.NewGorillaDialer(nil),
	}
}

// ASRV2Session represents one streaming recognition session.
type ASRV2Session struct {
	conn   transport.WSConn
	client *Client
	cfg    ASRV2Config
	reqID  string

	resultCh chan *ASRV2Result
	errCh    chan error
	closed   chan struct{}
	recvDone chan struct{}

	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// OpenStreamSession opens a SAUC V2 WebSocket session.
func (s *ASRServiceV2) OpenStreamSession(ctx context.Context, cfg *ASRV2Config) (*ASRV2Session, error) {
	if cfg == nil {
		return nil, newAPIError(CodeParamError, "config is nil")
	}

	normalized, err := normalizeASRV2Config(*cfg)
	if err != nil {
		return nil, err
	}

	resourceID := s.client.resolveResourceID(normalized.ResourceID, ResourceASRStreamV2)
	connectID := util.NewReqID("asr")

	endpoint := strings.TrimRight(s.client.config.wsURL, "/") + "/api/v3/sauc/bigmodel"
	headers := auth.BuildV2WSHeaders(s.client.authCredentials(), resourceID, connectID)

	conn, resp, err := s.dialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		return nil, wsConnectError(err, resp, responseMetadata{ReqID: connectID, ConnectID: connectID})
	}

	session := &ASRV2Session{
		conn:     conn,
		client:   s.client,
		cfg:      normalized,
		reqID:    connectID,
		resultCh: make(chan *ASRV2Result, 64),
		errCh:    make(chan error, 1),
		closed:   make(chan struct{}),
		recvDone: make(chan struct{}),
	}

	go session.receiveLoop()

	if err := session.sendSessionStart(ctx); err != nil {
		_ = session.Close()
		return nil, wrapError(err, "send session start")
	}

	return session, nil
}

// SendAudio sends one audio chunk.
//
// isLast=true marks the last frame (flags=2).
func (s *ASRV2Session) SendAudio(ctx context.Context, audio []byte, isLast bool) error {
	if len(audio) == 0 && !isLast {
		return newAPIError(CodeParamError, "audio payload is empty")
	}
	if err := s.guardContext(ctx); err != nil {
		return err
	}

	packet, err := protocol.BuildAudioOnly(audio, isLast)
	if err != nil {
		return wrapError(err, "encode audio frame")
	}

	return s.writeBinary(packet)
}

// Recv yields recognition results as a stream.
func (s *ASRV2Session) Recv() iter.Seq2[*ASRV2Result, error] {
	return func(yield func(*ASRV2Result, error) bool) {
		results := s.resultCh
		errs := s.errCh

		for results != nil || errs != nil {
			select {
			case r, ok := <-results:
				if !ok {
					results = nil
					continue
				}
				if !yield(r, nil) {
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

// Close closes the session.
func (s *ASRV2Session) Close() error {
	s.closeOnce.Do(func() {
		// Best-effort finish event; failures should not block close.
		_ = s.sendSessionFinish(context.Background())

		close(s.closed)
		s.closeErr = s.conn.Close()

		select {
		case <-s.recvDone:
		case <-time.After(2 * time.Second):
		}
	})

	return s.closeErr
}

func (s *ASRV2Session) sendSessionStart(ctx context.Context) error {
	requestConfig := resolvedASRV2RequestConfig(s.cfg)
	corpus, err := buildASRV2WireCorpus(requestConfig.Corpus)
	if err != nil {
		return wrapError(err, "marshal corpus context")
	}
	req := asrV2StartPayload{
		User: resolvedASRV2UserConfig(s.cfg, s.client.config.userID),
		Audio: asrV2StartAudio{
			Format:     s.cfg.Format,
			SampleRate: s.cfg.SampleRate,
			Channel:    resolvedChannel(s.cfg),
			Bits:       resolvedBits(s.cfg),
			Language:   s.cfg.Language,
			Codec:      s.cfg.Codec,
		},
		Request: asrV2WireRequest{
			ReqID:                  s.reqID,
			Sequence:               1,
			ModelName:              requestConfig.ModelName,
			EnableNonstream:        requestConfig.EnableNonstream,
			EnableITN:              requestConfig.EnableITN,
			EnableSpeakerInfo:      requestConfig.EnableSpeakerInfo,
			SSDVersion:             requestConfig.SSDVersion,
			EnablePunc:             requestConfig.EnablePunc,
			EnableDDC:              requestConfig.EnableDDC,
			OutputZHVariant:        requestConfig.OutputZHVariant,
			EnableAutoLanguage:     requestConfig.EnableAutoLanguage,
			ShowUtterances:         requestConfig.ShowUtterances,
			ShowSpeechRate:         requestConfig.ShowSpeechRate,
			ShowVolume:             requestConfig.ShowVolume,
			EnableLanguageID:       requestConfig.EnableLanguageID,
			EnableEmotionDetection: requestConfig.EnableEmotionDetection,
			EnableGenderDetection:  requestConfig.EnableGenderDetection,
			ResultType:             normalizedResultType(requestConfig.ResultType),
			EnableAccelerateText:   requestConfig.EnableAccelerateText,
			AccelerateScore:        requestConfig.AccelerateScore,
			VADSegmentDuration:     requestConfig.VADSegmentDuration,
			EndWindowSize:          requestConfig.EndWindowSize,
			ForceToSpeechTime:      requestConfig.ForceToSpeechTime,
			SensitiveWordsFilter:   requestConfig.SensitiveWordsFilter,
			EnablePOIFC:            requestConfig.EnablePOIFC,
			EnableMusicFC:          requestConfig.EnableMusicFC,
			Corpus:                 corpus,
			EnableDiarization:      s.cfg.EnableDiarization,
			SpeakerNum:             s.cfg.SpeakerNum,
			Hotwords:               s.cfg.Hotwords,
		},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return wrapError(err, "marshal start payload")
	}

	packet, err := protocol.BuildFullClientJSON(jsonBody)
	if err != nil {
		return wrapError(err, "encode start frame")
	}

	if err := s.guardContext(ctx); err != nil {
		return err
	}
	return s.writeBinary(packet)
}

type asrV2StartPayload struct {
	User    ASRV2UserConfig  `json:"user"`
	Audio   asrV2StartAudio  `json:"audio"`
	Request asrV2WireRequest `json:"request"`
}

type asrV2StartAudio struct {
	Format     AudioFormat     `json:"format"`
	SampleRate SampleRate      `json:"sample_rate"`
	Channel    int             `json:"channel"`
	Bits       int             `json:"bits"`
	Language   Language        `json:"language,omitempty"`
	Codec      ASRV2AudioCodec `json:"codec,omitempty"`
}

type asrV2WireRequest struct {
	ReqID                  string           `json:"reqid"`
	Sequence               int              `json:"sequence"`
	ModelName              string           `json:"model_name,omitempty"`
	EnableNonstream        *bool            `json:"enable_nonstream,omitempty"`
	EnableITN              *bool            `json:"enable_itn,omitempty"`
	EnableSpeakerInfo      *bool            `json:"enable_speaker_info,omitempty"`
	SSDVersion             string           `json:"ssd_version,omitempty"`
	EnablePunc             *bool            `json:"enable_punc,omitempty"`
	EnableDDC              *bool            `json:"enable_ddc,omitempty"`
	OutputZHVariant        string           `json:"output_zh_variant,omitempty"`
	EnableAutoLanguage     *bool            `json:"enable_auto_lang,omitempty"`
	ShowUtterances         *bool            `json:"show_utterances,omitempty"`
	ShowSpeechRate         *bool            `json:"show_speech_rate,omitempty"`
	ShowVolume             *bool            `json:"show_volume,omitempty"`
	EnableLanguageID       *bool            `json:"enable_lid,omitempty"`
	EnableEmotionDetection *bool            `json:"enable_emotion_detection,omitempty"`
	EnableGenderDetection  *bool            `json:"enable_gender_detection,omitempty"`
	ResultType             string           `json:"result_type"`
	EnableAccelerateText   *bool            `json:"enable_accelerate_text,omitempty"`
	AccelerateScore        *int             `json:"accelerate_score,omitempty"`
	VADSegmentDuration     *int             `json:"vad_segment_duration,omitempty"`
	EndWindowSize          *int             `json:"end_window_size,omitempty"`
	ForceToSpeechTime      *int             `json:"force_to_speech_time,omitempty"`
	SensitiveWordsFilter   *string          `json:"sensitive_words_filter,omitempty"`
	EnablePOIFC            *bool            `json:"enable_poi_fc,omitempty"`
	EnableMusicFC          *bool            `json:"enable_music_fc,omitempty"`
	Corpus                 *asrV2WireCorpus `json:"corpus,omitempty"`
	EnableDiarization      bool             `json:"enable_diarization,omitempty"`
	SpeakerNum             int              `json:"speaker_num,omitempty"`
	Hotwords               []string         `json:"hotwords,omitempty"`
}

type asrV2WireCorpus struct {
	BoostingTableName string `json:"boosting_table_name,omitempty"`
	BoostingTableID   string `json:"boosting_table_id,omitempty"`
	CorrectTableName  string `json:"correct_table_name,omitempty"`
	CorrectTableID    string `json:"correct_table_id,omitempty"`
	Context           string `json:"context,omitempty"`
}

type asrV2WireCorpusContext struct {
	Hotwords     []ASRV2Hotword       `json:"hotwords,omitempty"`
	CorrectWords asrV2WordCorrections `json:"correct_words,omitempty"`
	ContextType  string               `json:"context_type,omitempty"`
	ContextData  []ASRV2ContextEntry  `json:"context_data,omitempty"`
}

type asrV2WordCorrections []ASRV2WordCorrection

func (corrections asrV2WordCorrections) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, correction := range corrections {
		if i > 0 {
			buf.WriteByte(',')
		}
		source, err := json.Marshal(correction.Source)
		if err != nil {
			return nil, err
		}
		target, err := json.Marshal(correction.Target)
		if err != nil {
			return nil, err
		}
		buf.Write(source)
		buf.WriteByte(':')
		buf.Write(target)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func buildASRV2WireCorpus(config *ASRV2CorpusConfig) (*asrV2WireCorpus, error) {
	if config == nil {
		return nil, nil
	}
	corpus := &asrV2WireCorpus{
		BoostingTableName: config.BoostingTableName,
		BoostingTableID:   config.BoostingTableID,
		CorrectTableName:  config.CorrectTableName,
		CorrectTableID:    config.CorrectTableID,
	}
	if config.Context != nil {
		contextJSON, err := json.Marshal(asrV2WireCorpusContext{
			Hotwords:     config.Context.Hotwords,
			CorrectWords: asrV2WordCorrections(config.Context.CorrectWords),
			ContextType:  config.Context.ContextType,
			ContextData:  config.Context.ContextData,
		})
		if err != nil {
			return nil, err
		}
		corpus.Context = string(contextJSON)
	}
	return corpus, nil
}

func (s *ASRV2Session) sendSessionFinish(ctx context.Context) error {
	finishBody, err := json.Marshal(asrV2FinishPayload{
		Event: 2,
		ReqID: s.reqID,
	})
	if err != nil {
		return wrapError(err, "marshal finish payload")
	}

	packet, err := protocol.BuildFullClientJSON(finishBody)
	if err != nil {
		return wrapError(err, "encode finish frame")
	}

	if err := s.guardContext(ctx); err != nil {
		return err
	}
	return s.writeBinary(packet)
}

type asrV2FinishPayload struct {
	Event int    `json:"event"`
	ReqID string `json:"reqid"`
}

func (s *ASRV2Session) writeBinary(packet []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.isClosed() {
		return newAPIError(CodeServerError, "session already closed")
	}

	return s.conn.WriteMessage(websocket.BinaryMessage, packet)
}

func (s *ASRV2Session) receiveLoop() {
	defer close(s.recvDone)
	defer close(s.resultCh)
	defer close(s.errCh)

	for {
		if s.isClosed() {
			return
		}

		msgType, payload, err := s.conn.ReadMessage()
		if err != nil {
			if s.isClosed() {
				return
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			s.pushErr(wrapError(err, "websocket read"))
			return
		}

		switch msgType {
		case websocket.TextMessage:
			s.pushErr(withErrorMetadata(parseWSErrorPayload(payload, 0), responseMetadata{ReqID: s.reqID, ConnectID: s.reqID}))
			return
		case websocket.BinaryMessage:
			frame, err := protocol.ParseServerFrame(payload)
			if err != nil {
				s.pushErr(wrapError(err, "parse server frame"))
				continue
			}

			switch frame.MessageType {
			case protocol.MessageTypeFullServer:
				result, err := decodeASRV2Result(frame, s.reqID)
				if err != nil {
					s.pushErr(err)
					continue
				}
				if result == nil {
					continue
				}
				select {
				case s.resultCh <- result:
				case <-s.closed:
					return
				}
			case protocol.MessageTypeError:
				s.pushErr(withErrorMetadata(parseWSErrorPayload(frame.Payload, frame.ErrorCode), responseMetadata{ReqID: s.reqID, ConnectID: s.reqID}))
				return
			default:
				// Ignore other frame types in this migration scope.
			}
		default:
			// Ignore unknown message types.
		}
	}
}

func decodeASRV2Result(frame *protocol.ParsedFrame, fallbackReqID string) (*ASRV2Result, error) {
	var payload struct {
		ReqID     string `json:"reqid"`
		TraceID   string `json:"trace_id"`
		LogID     string `json:"log_id"`
		LogIDAlt  string `json:"logid"`
		ConnectID string `json:"connect_id"`
		Code      int    `json:"code"`
		Message   string `json:"message"`
		AudioInfo struct {
			Duration int `json:"duration"`
		} `json:"audio_info"`
		Result struct {
			Text       string `json:"text"`
			Utterances []struct {
				Text       string  `json:"text"`
				StartTime  int     `json:"start_time"`
				EndTime    int     `json:"end_time"`
				Definite   bool    `json:"definite"`
				SpeakerID  string  `json:"speaker_id"`
				Confidence float64 `json:"confidence"`
				Words      []struct {
					Text      string  `json:"text"`
					StartTime int     `json:"start_time"`
					EndTime   int     `json:"end_time"`
					Conf      float64 `json:"conf"`
				} `json:"words"`
			} `json:"utterances"`
		} `json:"result"`
	}

	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return nil, wrapError(err, "unmarshal asr result")
	}

	if payload.Code != 0 && payload.Code != CodeSuccess && payload.Code != CodeASRSuccess {
		return nil, &Error{
			Code:      payload.Code,
			Message:   payload.Message,
			ReqID:     payload.ReqID,
			TraceID:   payload.TraceID,
			LogID:     firstNonEmpty(payload.LogID, payload.LogIDAlt),
			ConnectID: payload.ConnectID,
		}
	}

	if payload.Result.Text == "" && len(payload.Result.Utterances) == 0 {
		// This can be an intermediate control frame.
		return nil, nil
	}

	utterances := make([]ASRV2Utterance, 0, len(payload.Result.Utterances))
	isFinal := frame.Flags == protocol.FlagNegativeSequence || frame.Flags == protocol.FlagNegativeWithSeq
	for _, u := range payload.Result.Utterances {
		words := make([]ASRV2Word, 0, len(u.Words))
		for _, w := range u.Words {
			words = append(words, ASRV2Word{
				Text:      w.Text,
				StartTime: w.StartTime,
				EndTime:   w.EndTime,
				Conf:      w.Conf,
			})
		}

		utterances = append(utterances, ASRV2Utterance{
			Text:       u.Text,
			StartTime:  u.StartTime,
			EndTime:    u.EndTime,
			Definite:   u.Definite,
			SpeakerID:  u.SpeakerID,
			Confidence: u.Confidence,
			Words:      words,
		})

		if u.Definite {
			isFinal = true
		}
	}

	reqID := payload.ReqID
	if reqID == "" {
		reqID = fallbackReqID
	}

	return &ASRV2Result{
		Text:       payload.Result.Text,
		Utterances: utterances,
		Duration:   payload.AudioInfo.Duration,
		IsFinal:    isFinal,
		ReqID:      reqID,
		TraceID:    payload.TraceID,
		LogID:      firstNonEmpty(payload.LogID, payload.LogIDAlt),
		ConnectID:  firstNonEmpty(payload.ConnectID, fallbackReqID),
	}, nil
}

func parseWSErrorPayload(payload []byte, fallbackCode uint32) error {
	e, ok := parseAPIErrorPayload(payload)
	if !ok {
		msg := string(payload)
		if msg == "" {
			msg = "unknown websocket error"
		}
		code := int(fallbackCode)
		if code == 0 {
			code = CodeServerError
		}
		return &Error{Code: code, Message: msg}
	}

	meta := parseResponseMetadata(payload, responseMetadata{})

	return &Error{
		Code:      apiErrorPayloadCode(e, int(fallbackCode)),
		Message:   apiErrorPayloadMessage(e, "websocket error"),
		ReqID:     meta.ReqID,
		TraceID:   meta.TraceID,
		LogID:     meta.LogID,
		ConnectID: meta.ConnectID,
	}
}

func normalizeASRV2Config(cfg ASRV2Config) (ASRV2Config, error) {
	if cfg.Format == "" {
		cfg.Format = FormatPCM
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = SampleRate16000
	}
	if cfg.Channel == 0 && cfg.Channels == 0 {
		cfg.Channel = 1
	}
	if cfg.Bits == 0 {
		cfg.Bits = 16
	}

	if err := util.ValidateFormat(string(cfg.Format)); err != nil {
		return cfg, newAPIError(CodeParamError, err.Error())
	}
	if err := util.ValidateSampleRate(int(cfg.SampleRate)); err != nil {
		return cfg, newAPIError(CodeParamError, err.Error())
	}
	if err := util.ValidateChannel(resolvedChannel(cfg)); err != nil {
		return cfg, newAPIError(CodeParamError, err.Error())
	}
	if err := util.ValidateBits(resolvedBits(cfg)); err != nil {
		return cfg, newAPIError(CodeParamError, err.Error())
	}
	if cfg.Codec != "" && cfg.Codec != ASRV2AudioCodecRaw && cfg.Codec != ASRV2AudioCodecOpus {
		return cfg, newAPIError(CodeParamError, "codec must be raw or opus")
	}
	request := resolvedASRV2RequestConfig(cfg)
	if err := util.ValidateResultType(request.ResultType); err != nil {
		return cfg, newAPIError(CodeParamError, err.Error())
	}
	if request.AccelerateScore != nil && (*request.AccelerateScore < 0 || *request.AccelerateScore > 20) {
		return cfg, newAPIError(CodeParamError, "accelerate_score must be between 0 and 20")
	}
	if request.VADSegmentDuration != nil && *request.VADSegmentDuration < 0 {
		return cfg, newAPIError(CodeParamError, "vad_segment_duration must not be negative")
	}
	if request.EndWindowSize != nil && *request.EndWindowSize < 200 {
		return cfg, newAPIError(CodeParamError, "end_window_size must be at least 200 milliseconds")
	}
	if request.ForceToSpeechTime != nil {
		if *request.ForceToSpeechTime < 0 {
			return cfg, newAPIError(CodeParamError, "force_to_speech_time must not be negative")
		}
		if request.EndWindowSize == nil {
			return cfg, newAPIError(CodeParamError, "force_to_speech_time requires end_window_size")
		}
	}
	if request.OutputZHVariant != "" && request.OutputZHVariant != "traditional" && request.OutputZHVariant != "tw" && request.OutputZHVariant != "hk" {
		return cfg, newAPIError(CodeParamError, "output_zh_variant must be traditional, tw, or hk")
	}

	return cfg, nil
}

func resolvedASRV2UserConfig(cfg ASRV2Config, fallbackUID string) ASRV2UserConfig {
	user := ASRV2UserConfig{}
	if cfg.User != nil {
		user = *cfg.User
	}
	if user.UID == "" {
		user.UID = fallbackUID
	}
	return user
}

func resolvedASRV2RequestConfig(cfg ASRV2Config) ASRV2RequestConfig {
	request := ASRV2RequestConfig{}
	if cfg.Request != nil {
		request = *cfg.Request
	}
	if request.EnableITN == nil && cfg.EnableITN {
		value := true
		request.EnableITN = &value
	}
	if request.EnablePunc == nil && cfg.EnablePunc {
		value := true
		request.EnablePunc = &value
	}
	if request.ResultType == "" {
		request.ResultType = cfg.ResultType
	}
	if request.ShowUtterances == nil {
		value := true
		request.ShowUtterances = &value
	}
	return request
}

func normalizedResultType(rt string) string {
	v := strings.ToLower(strings.TrimSpace(rt))
	if v == "full" {
		return "full"
	}
	return "single"
}

func resolvedChannel(cfg ASRV2Config) int {
	if cfg.Channel > 0 {
		return cfg.Channel
	}
	if cfg.Channels > 0 {
		return cfg.Channels
	}
	return 1
}

func resolvedBits(cfg ASRV2Config) int {
	if cfg.Bits > 0 {
		return cfg.Bits
	}
	return 16
}

func (s *ASRV2Session) guardContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if s.isClosed() {
		return newAPIError(CodeServerError, "session already closed")
	}
	return nil
}

func (s *ASRV2Session) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

func (s *ASRV2Session) pushErr(err error) {
	if err == nil {
		return
	}
	select {
	case s.errCh <- err:
	default:
	}
}

func wsConnectError(baseErr error, resp *http.Response, metas ...responseMetadata) error {
	if resp == nil || resp.Body == nil {
		return wrapError(baseErr, "websocket connect failed")
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	logID := resp.Header.Get("X-Tt-Logid")
	if logID == "" {
		logID = resp.Header.Get("X-Tt-LogId")
	}

	if readErr == nil && len(body) > 0 {
		if parsed := parseAPIError(resp.StatusCode, body, logID); parsed != nil {
			if len(metas) > 0 {
				parsed = withErrorMetadata(parsed, metas[0])
			}
			return wrapError(parsed, "websocket connect failed")
		}
		return fmt.Errorf("websocket connect failed: %w (status=%s, body=%s)", baseErr, resp.Status, string(body))
	}

	return fmt.Errorf("websocket connect failed: %w (status=%s)", baseErr, resp.Status)
}
