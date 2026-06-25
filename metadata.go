package doubaospeech

import "encoding/json"

type responseMetadata struct {
	ReqID     string
	TraceID   string
	LogID     string
	ConnectID string
}

type responseMetadataSetter interface {
	setResponseMetadata(responseMetadata)
}

func (m responseMetadata) withFallback(fallback responseMetadata) responseMetadata {
	if m.ReqID == "" {
		m.ReqID = fallback.ReqID
	}
	if m.TraceID == "" {
		m.TraceID = fallback.TraceID
	}
	if m.LogID == "" {
		m.LogID = fallback.LogID
	}
	if m.ConnectID == "" {
		m.ConnectID = fallback.ConnectID
	}
	return m
}

func applyErrorMetadata(e *Error, meta responseMetadata) {
	if e == nil {
		return
	}
	if e.ReqID == "" {
		e.ReqID = meta.ReqID
	}
	if e.TraceID == "" {
		e.TraceID = meta.TraceID
	}
	if e.LogID == "" {
		e.LogID = meta.LogID
	}
	if e.ConnectID == "" {
		e.ConnectID = meta.ConnectID
	}
}

func withErrorMetadata(err error, meta responseMetadata) error {
	if err == nil {
		return nil
	}
	if apiErr, ok := AsError(err); ok {
		applyErrorMetadata(apiErr, meta)
	}
	return err
}

type responseMetadataPayload struct {
	ReqID     string `json:"reqid"`
	RequestID string `json:"request_id"`
	TraceID   string `json:"trace_id"`
	LogID     string `json:"log_id"`
	LogIDAlt  string `json:"logid"`
	ConnectID string `json:"connect_id"`
}

func (p responseMetadataPayload) withFallback(fallback responseMetadataPayload) responseMetadataPayload {
	if firstNonEmpty(p.ReqID, p.RequestID) == "" {
		p.ReqID = fallback.ReqID
		p.RequestID = fallback.RequestID
	}
	if p.TraceID == "" {
		p.TraceID = fallback.TraceID
	}
	if firstNonEmpty(p.LogID, p.LogIDAlt) == "" {
		p.LogID = fallback.LogID
		p.LogIDAlt = fallback.LogIDAlt
	}
	if p.ConnectID == "" {
		p.ConnectID = fallback.ConnectID
	}
	return p
}

type nestedResponseMetadataPayload struct {
	Header *responseMetadataPayload `json:"header"`
}

func responseMetadataFromPayload(p responseMetadataPayload) responseMetadata {
	return responseMetadata{
		ReqID:     firstNonEmpty(p.ReqID, p.RequestID),
		TraceID:   p.TraceID,
		LogID:     firstNonEmpty(p.LogID, p.LogIDAlt),
		ConnectID: p.ConnectID,
	}
}

func parseResponseMetadata(data []byte, fallback responseMetadata) responseMetadata {
	if len(data) == 0 {
		return fallback
	}

	var payload responseMetadataPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fallback
	}
	meta := responseMetadataFromPayload(payload)

	var nested nestedResponseMetadataPayload
	if err := json.Unmarshal(data, &nested); err == nil && nested.Header != nil {
		meta = meta.withFallback(responseMetadataFromPayload(*nested.Header))
	}

	return meta.withFallback(fallback)
}
