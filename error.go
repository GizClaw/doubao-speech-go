package doubaospeech

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Error is the unified error model.
type Error struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	TraceID    string `json:"trace_id,omitempty"`
	LogID      string `json:"log_id,omitempty"`
	ConnectID  string `json:"connect_id,omitempty"`
	HTTPStatus int    `json:"-"`
	ReqID      string `json:"reqid,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf(
		"doubaospeech: %s (code=%d, reqid=%s, trace_id=%s, log_id=%s, connect_id=%s, http_status=%d)",
		e.Message,
		e.Code,
		e.ReqID,
		e.TraceID,
		e.LogID,
		e.ConnectID,
		e.HTTPStatus,
	)
}

func (e *Error) IsAuthError() bool {
	return e.HTTPStatus == http.StatusUnauthorized || e.HTTPStatus == http.StatusForbidden || e.Code == CodeAuthError
}

func (e *Error) IsRateLimit() bool {
	return e.HTTPStatus == http.StatusTooManyRequests || e.Code == CodeRateLimit
}

func (e *Error) IsQuotaExceeded() bool {
	return e.HTTPStatus == http.StatusPaymentRequired || e.Code == CodeQuotaExceed
}

func (e *Error) IsInvalidParam() bool {
	return e.HTTPStatus == http.StatusBadRequest || e.Code == CodeParamError
}

func (e *Error) IsServerError() bool {
	return e.HTTPStatus >= http.StatusInternalServerError || e.Code == CodeServerError
}

func (e *Error) Retryable() bool {
	return e.IsRateLimit() || e.IsServerError()
}

// AsError converts an error to *Error.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

const (
	CodeSuccess     = 3000
	CodeParamError  = 3001
	CodeAuthError   = 3002
	CodeRateLimit   = 3003
	CodeQuotaExceed = 3004
	CodeServerError = 3005
	CodeASRSuccess  = 1000
)

type apiErrorPayload struct {
	Code       int    `json:"code"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Error      string `json:"error"`
	responseMetadataPayload
}

type apiErrorEnvelope struct {
	Header *apiErrorPayload `json:"header"`
}

func parseAPIErrorPayload(data []byte) (apiErrorPayload, bool) {
	var payload apiErrorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return apiErrorPayload{}, false
	}

	var envelope apiErrorEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Header != nil {
		payload = payload.withFallback(*envelope.Header)
	}

	return payload, true
}

func (p apiErrorPayload) withFallback(fallback apiErrorPayload) apiErrorPayload {
	if p.Code == 0 {
		p.Code = fallback.Code
	}
	if p.StatusCode == 0 {
		p.StatusCode = fallback.StatusCode
	}
	if p.Message == "" {
		p.Message = fallback.Message
	}
	if p.Error == "" {
		p.Error = fallback.Error
	}
	p.responseMetadataPayload = p.responseMetadataPayload.withFallback(fallback.responseMetadataPayload)
	return p
}

func apiErrorPayloadCode(payload apiErrorPayload, fallback int) int {
	code := payload.Code
	if code == 0 {
		code = payload.StatusCode
	}
	if code == 0 {
		code = fallback
	}
	if code == 0 {
		code = CodeServerError
	}
	return code
}

func apiErrorPayloadMessage(payload apiErrorPayload, fallback string) string {
	msg := payload.Message
	if msg == "" {
		msg = payload.Error
	}
	if msg == "" {
		msg = fallback
	}
	return msg
}

func parseAPIError(statusCode int, body []byte, logID string) error {
	baseMeta := responseMetadata{LogID: logID}
	if len(body) == 0 {
		return &Error{
			Code:       statusCode,
			Message:    http.StatusText(statusCode),
			HTTPStatus: statusCode,
			LogID:      baseMeta.LogID,
		}
	}

	payload, ok := parseAPIErrorPayload(body)
	if !ok {
		return &Error{
			Code:       statusCode,
			Message:    string(body),
			HTTPStatus: statusCode,
			LogID:      baseMeta.LogID,
		}
	}

	meta := parseResponseMetadata(body, baseMeta)

	return &Error{
		Code:       apiErrorPayloadCode(payload, statusCode),
		Message:    apiErrorPayloadMessage(payload, http.StatusText(statusCode)),
		TraceID:    meta.TraceID,
		LogID:      meta.LogID,
		ConnectID:  meta.ConnectID,
		HTTPStatus: statusCode,
		ReqID:      meta.ReqID,
	}
}

func newAPIError(code int, message string) *Error {
	if code == 0 {
		code = CodeServerError
	}
	return &Error{Code: code, Message: message}
}

func wrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}
