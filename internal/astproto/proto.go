package astproto

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	EventStartSession    int32 = 100
	EventFinishSession   int32 = 102
	EventTaskRequest     int32 = 200
	EventUpdateConfig    int32 = 201
	EventSessionStarted  int32 = 150
	EventSessionCanceled int32 = 151
	EventSessionFinished int32 = 152
	EventSessionFailed   int32 = 153
	EventUsageResponse   int32 = 154
	EventAudioMuted      int32 = 250

	EventTTSSentenceStart            int32 = 350
	EventTTSSentenceEnd              int32 = 351
	EventTTSResponse                 int32 = 352
	EventSourceSubtitleStart         int32 = 650
	EventSourceSubtitleResponse      int32 = 651
	EventSourceSubtitleEnd           int32 = 652
	EventTranslationSubtitleStart    int32 = 653
	EventTranslationSubtitleResponse int32 = 654
	EventTranslationSubtitleEnd      int32 = 655
)

const StatusSuccess = 20000000

type RequestMeta struct {
	SessionID string
}

type User struct {
	UID        string
	DID        string
	Platform   string
	SDKVersion string
	AppVersion string
}

type Audio struct {
	Data       []byte
	URL        string
	URLType    string
	Format     string
	Codec      string
	Language   string
	Rate       int32
	Bits       int32
	Channel    int32
	BinaryData []byte
}

type Corpus struct {
	Context               string
	BoostingTableID       string
	BoostingTableName     string
	CorrectTableID        string
	CorrectTableName      string
	HotWordsList          []string
	GlossaryList          map[string]string
	CorrectWords          string
	RegexCorrectTableID   string
	RegexCorrectTableName string
	GlossaryTableID       string
	GlossaryTableName     string
}

type ReqParams struct {
	Mode                       string
	SourceLanguage             string
	TargetLanguage             string
	SpeakerID                  string
	IsCustomSpeaker            *bool
	TTSResourceID              string
	SpeechRate                 int32
	EnableSourceLanguageDetect *bool
	Corpus                     *Corpus
}

type TranslateRequest struct {
	RequestMeta *RequestMeta
	Event       int32
	User        *User
	SourceAudio *Audio
	TargetAudio *Audio
	Request     *ReqParams
	Denoise     *bool
}

type BillingItem struct {
	Unit     string
	Quantity float32
}

type Billing struct {
	Items        []BillingItem
	DurationMsec int64
	WordCount    int64
}

type ResponseMeta struct {
	SessionID  string
	Sequence   int32
	StatusCode int32
	Message    string
	Billing    *Billing
}

type TranslateResponse struct {
	ResponseMeta     *ResponseMeta
	Event            int32
	Data             []byte
	Text             string
	StartTime        int32
	EndTime          int32
	SpeakerChanged   bool
	MutedDuration    int32
	DetectedLanguage string
}

func MarshalRequest(req *TranslateRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("translate request is nil")
	}
	var out []byte
	if req.RequestMeta != nil {
		out = appendMessage(out, 1, marshalRequestMeta(req.RequestMeta))
	}
	if req.Event != 0 {
		out = appendInt32(out, 2, req.Event)
	}
	if req.User != nil {
		out = appendMessage(out, 3, marshalUser(req.User))
	}
	if req.SourceAudio != nil {
		out = appendMessage(out, 4, marshalAudio(req.SourceAudio))
	}
	if req.TargetAudio != nil {
		out = appendMessage(out, 5, marshalAudio(req.TargetAudio))
	}
	if req.Request != nil {
		out = appendMessage(out, 6, marshalReqParams(req.Request))
	}
	if req.Denoise != nil {
		out = appendBool(out, 7, *req.Denoise)
	}
	return out, nil
}

func UnmarshalRequest(data []byte) (*TranslateRequest, error) {
	req := &TranslateRequest{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			meta, err := unmarshalRequestMeta(b)
			if err != nil {
				return nil, err
			}
			req.RequestMeta = meta
			data = r
		case 2:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			req.Event = int32(v)
			data = r
		case 3:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			user, err := unmarshalUser(b)
			if err != nil {
				return nil, err
			}
			req.User = user
			data = r
		case 4:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			audio, err := unmarshalAudio(b)
			if err != nil {
				return nil, err
			}
			req.SourceAudio = audio
			data = r
		case 5:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			audio, err := unmarshalAudio(b)
			if err != nil {
				return nil, err
			}
			req.TargetAudio = audio
			data = r
		case 6:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			params, err := unmarshalReqParams(b)
			if err != nil {
				return nil, err
			}
			req.Request = params
			data = r
		case 7:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			denoise := v != 0
			req.Denoise = &denoise
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return req, nil
}

func MarshalResponse(resp *TranslateResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("translate response is nil")
	}
	var out []byte
	if resp.ResponseMeta != nil {
		out = appendMessage(out, 1, marshalResponseMeta(resp.ResponseMeta))
	}
	if resp.Event != 0 {
		out = appendInt32(out, 2, resp.Event)
	}
	out = appendBytesField(out, 3, resp.Data)
	out = appendString(out, 4, resp.Text)
	if resp.StartTime != 0 {
		out = appendInt32(out, 5, resp.StartTime)
	}
	if resp.EndTime != 0 {
		out = appendInt32(out, 6, resp.EndTime)
	}
	if resp.SpeakerChanged {
		out = appendBool(out, 7, resp.SpeakerChanged)
	}
	if resp.MutedDuration != 0 {
		out = appendInt32(out, 8, resp.MutedDuration)
	}
	out = appendString(out, 9, resp.DetectedLanguage)
	return out, nil
}

func UnmarshalResponse(data []byte) (*TranslateResponse, error) {
	resp := &TranslateResponse{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, fmt.Errorf("response_meta: %w", err)
			}
			meta, err := unmarshalResponseMeta(b)
			if err != nil {
				return nil, err
			}
			resp.ResponseMeta = meta
			data = r
		case 2:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			resp.Event = int32(v)
			data = r
		case 3:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			resp.Data = b
			data = r
		case 4:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			resp.Text = string(b)
			data = r
		case 5:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			resp.StartTime = int32(v)
			data = r
		case 6:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			resp.EndTime = int32(v)
			data = r
		case 7:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			resp.SpeakerChanged = v != 0
			data = r
		case 8:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			resp.MutedDuration = int32(v)
			data = r
		case 9:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			resp.DetectedLanguage = string(b)
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return resp, nil
}

func marshalRequestMeta(meta *RequestMeta) []byte {
	var out []byte
	out = appendString(out, 6, meta.SessionID)
	return out
}

func marshalResponseMeta(meta *ResponseMeta) []byte {
	var out []byte
	out = appendString(out, 1, meta.SessionID)
	if meta.Sequence != 0 {
		out = appendInt32(out, 2, meta.Sequence)
	}
	if meta.StatusCode != 0 {
		out = appendInt32(out, 3, meta.StatusCode)
	}
	out = appendString(out, 4, meta.Message)
	if meta.Billing != nil {
		out = appendMessage(out, 5, marshalBilling(meta.Billing))
	}
	return out
}

func marshalBilling(billing *Billing) []byte {
	var out []byte
	for _, item := range billing.Items {
		out = appendMessage(out, 1, marshalBillingItem(item))
	}
	if billing.DurationMsec != 0 {
		out = appendTag(out, 2, 0)
		out = appendVarint(out, uint64(billing.DurationMsec))
	}
	if billing.WordCount != 0 {
		out = appendTag(out, 3, 0)
		out = appendVarint(out, uint64(billing.WordCount))
	}
	return out
}

func marshalBillingItem(item BillingItem) []byte {
	var out []byte
	out = appendString(out, 1, item.Unit)
	if item.Quantity != 0 {
		out = appendTag(out, 2, 5)
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], math.Float32bits(item.Quantity))
		out = append(out, b[:]...)
	}
	return out
}

func marshalUser(user *User) []byte {
	var out []byte
	out = appendString(out, 1, user.UID)
	out = appendString(out, 2, user.DID)
	out = appendString(out, 3, user.Platform)
	out = appendString(out, 4, user.SDKVersion)
	out = appendString(out, 5, user.AppVersion)
	return out
}

func unmarshalRequestMeta(data []byte) (*RequestMeta, error) {
	meta := &RequestMeta{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 6:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			meta.SessionID = string(b)
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return meta, nil
}

func unmarshalUser(data []byte) (*User, error) {
	user := &User{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			user.UID = string(b)
			data = r
		case 2:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			user.DID = string(b)
			data = r
		case 3:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			user.Platform = string(b)
			data = r
		case 4:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			user.SDKVersion = string(b)
			data = r
		case 5:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			user.AppVersion = string(b)
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return user, nil
}

func unmarshalAudio(data []byte) (*Audio, error) {
	audio := &Audio{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			audio.Data = b
			data = r
		case 4:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			audio.Format = string(b)
			data = r
		case 5:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			audio.Codec = string(b)
			data = r
		case 7:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			audio.Rate = int32(v)
			data = r
		case 8:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			audio.Bits = int32(v)
			data = r
		case 9:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			audio.Channel = int32(v)
			data = r
		case 14:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			audio.BinaryData = b
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return audio, nil
}

func unmarshalReqParams(data []byte) (*ReqParams, error) {
	params := &ReqParams{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			params.Mode = string(b)
			data = r
		case 2:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			params.SourceLanguage = string(b)
			data = r
		case 3:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			params.TargetLanguage = string(b)
			data = r
		case 4:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			params.SpeakerID = string(b)
			data = r
		case 5:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			value := v != 0
			params.IsCustomSpeaker = &value
			data = r
		case 6:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			params.TTSResourceID = string(b)
			data = r
		case 7:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			params.SpeechRate = int32(v)
			data = r
		case 8:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			value := v != 0
			params.EnableSourceLanguageDetect = &value
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return params, nil
}

func marshalAudio(audio *Audio) []byte {
	var out []byte
	out = appendBytesField(out, 1, audio.Data)
	out = appendString(out, 2, audio.URL)
	out = appendString(out, 3, audio.URLType)
	out = appendString(out, 4, audio.Format)
	out = appendString(out, 5, audio.Codec)
	out = appendString(out, 6, audio.Language)
	if audio.Rate != 0 {
		out = appendInt32(out, 7, audio.Rate)
	}
	if audio.Bits != 0 {
		out = appendInt32(out, 8, audio.Bits)
	}
	if audio.Channel != 0 {
		out = appendInt32(out, 9, audio.Channel)
	}
	out = appendBytesField(out, 14, audio.BinaryData)
	return out
}

func marshalReqParams(req *ReqParams) []byte {
	var out []byte
	out = appendString(out, 1, req.Mode)
	out = appendString(out, 2, req.SourceLanguage)
	out = appendString(out, 3, req.TargetLanguage)
	out = appendString(out, 4, req.SpeakerID)
	if req.IsCustomSpeaker != nil {
		out = appendBool(out, 5, *req.IsCustomSpeaker)
	}
	out = appendString(out, 6, req.TTSResourceID)
	if req.SpeechRate != 0 {
		out = appendInt32(out, 7, req.SpeechRate)
	}
	if req.EnableSourceLanguageDetect != nil {
		out = appendBool(out, 8, *req.EnableSourceLanguageDetect)
	}
	if req.Corpus != nil {
		out = appendMessage(out, 100, marshalCorpus(req.Corpus))
	}
	return out
}

func marshalCorpus(corpus *Corpus) []byte {
	var out []byte
	out = appendString(out, 1, corpus.Context)
	out = appendString(out, 2, corpus.BoostingTableID)
	out = appendString(out, 3, corpus.BoostingTableName)
	out = appendString(out, 6, corpus.CorrectTableID)
	out = appendString(out, 7, corpus.CorrectTableName)
	for _, word := range corpus.HotWordsList {
		out = appendString(out, 9, word)
	}
	for k, v := range corpus.GlossaryList {
		var entry []byte
		entry = appendString(entry, 1, k)
		entry = appendString(entry, 2, v)
		out = appendMessage(out, 10, entry)
	}
	out = appendString(out, 11, corpus.CorrectWords)
	out = appendString(out, 12, corpus.RegexCorrectTableID)
	out = appendString(out, 13, corpus.RegexCorrectTableName)
	out = appendString(out, 14, corpus.GlossaryTableID)
	out = appendString(out, 15, corpus.GlossaryTableName)
	return out
}

func unmarshalResponseMeta(data []byte) (*ResponseMeta, error) {
	meta := &ResponseMeta{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			meta.SessionID = string(b)
			data = r
		case 2:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			meta.Sequence = int32(v)
			data = r
		case 3:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			meta.StatusCode = int32(v)
			data = r
		case 4:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			meta.Message = string(b)
			data = r
		case 5:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			billing, err := unmarshalBilling(b)
			if err != nil {
				return nil, err
			}
			meta.Billing = billing
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return meta, nil
}

func unmarshalBilling(data []byte) (*Billing, error) {
	billing := &Billing{}
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return nil, err
			}
			item, err := unmarshalBillingItem(b)
			if err != nil {
				return nil, err
			}
			billing.Items = append(billing.Items, item)
			data = r
		case 2:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			billing.DurationMsec = int64(v)
			data = r
		case 3:
			v, r, err := consumeVarint(data)
			if err != nil {
				return nil, err
			}
			billing.WordCount = int64(v)
			data = r
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return nil, err
			}
			data = r
		}
	}
	return billing, nil
}

func unmarshalBillingItem(data []byte) (BillingItem, error) {
	var item BillingItem
	for len(data) > 0 {
		field, wire, rest, err := consumeTag(data)
		if err != nil {
			return item, err
		}
		data = rest
		switch field {
		case 1:
			b, r, err := consumeBytes(data)
			if err != nil {
				return item, err
			}
			item.Unit = string(b)
			data = r
		case 2:
			if wire != 5 {
				return item, fmt.Errorf("billing quantity wire type = %d, want 5", wire)
			}
			if len(data) < 4 {
				return item, fmt.Errorf("truncated fixed32")
			}
			item.Quantity = math.Float32frombits(binary.LittleEndian.Uint32(data[:4]))
			data = data[4:]
		default:
			r, err := skipValue(wire, data)
			if err != nil {
				return item, err
			}
			data = r
		}
	}
	return item, nil
}

func appendTag(out []byte, field int, wire int) []byte {
	return appendVarint(out, uint64(field<<3|wire))
}

func appendInt32(out []byte, field int, v int32) []byte {
	out = appendTag(out, field, 0)
	return appendVarint(out, uint64(v))
}

func appendBool(out []byte, field int, v bool) []byte {
	out = appendTag(out, field, 0)
	if v {
		return append(out, 1)
	}
	return append(out, 0)
}

func appendString(out []byte, field int, v string) []byte {
	if v == "" {
		return out
	}
	return appendBytesField(out, field, []byte(v))
}

func appendMessage(out []byte, field int, msg []byte) []byte {
	return appendBytesField(out, field, msg)
}

func appendBytesField(out []byte, field int, v []byte) []byte {
	if len(v) == 0 {
		return out
	}
	out = appendTag(out, field, 2)
	out = appendVarint(out, uint64(len(v)))
	return append(out, v...)
}

func appendVarint(out []byte, v uint64) []byte {
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

func consumeTag(data []byte) (field int, wire int, rest []byte, err error) {
	v, rest, err := consumeVarint(data)
	if err != nil {
		return 0, 0, nil, err
	}
	field = int(v >> 3)
	wire = int(v & 0x7)
	if field <= 0 {
		return 0, 0, nil, fmt.Errorf("invalid field number %d", field)
	}
	return field, wire, rest, nil
}

func consumeVarint(data []byte) (uint64, []byte, error) {
	var v uint64
	for i, b := range data {
		if i == 10 {
			return 0, nil, fmt.Errorf("varint too long")
		}
		v |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return v, data[i+1:], nil
		}
	}
	return 0, nil, fmt.Errorf("truncated varint")
}

func consumeBytes(data []byte) ([]byte, []byte, error) {
	size, rest, err := consumeVarint(data)
	if err != nil {
		return nil, nil, err
	}
	if size > uint64(len(rest)) {
		return nil, nil, fmt.Errorf("length-delimited field size %d exceeds remaining %d", size, len(rest))
	}
	return rest[:size], rest[size:], nil
}

func skipValue(wire int, data []byte) ([]byte, error) {
	switch wire {
	case 0:
		_, rest, err := consumeVarint(data)
		return rest, err
	case 1:
		if len(data) < 8 {
			return nil, fmt.Errorf("truncated fixed64")
		}
		return data[8:], nil
	case 2:
		_, rest, err := consumeBytes(data)
		return rest, err
	case 5:
		if len(data) < 4 {
			return nil, fmt.Errorf("truncated fixed32")
		}
		return data[4:], nil
	default:
		return nil, fmt.Errorf("unsupported wire type %d", wire)
	}
}
