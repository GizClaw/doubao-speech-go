package doubaospeech

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
