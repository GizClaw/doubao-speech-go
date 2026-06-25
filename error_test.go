package doubaospeech

import (
	"net/http"
	"testing"
)

func TestParseAPIErrorNestedHeaderPayload(t *testing.T) {
	err := parseAPIError(http.StatusUnauthorized, []byte(`{"header":{"reqid":"req-nested-1","trace_id":"trace-nested-1","code":45000010,"message":"Invalid X-Api-Key"}}`), "log-nested-1")
	if err == nil {
		t.Fatalf("parseAPIError returned nil")
	}

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T", err)
	}

	if apiErr.Code != 45000010 {
		t.Fatalf("code = %d, want 45000010", apiErr.Code)
	}
	if apiErr.Message != "Invalid X-Api-Key" {
		t.Fatalf("message = %q, want %q", apiErr.Message, "Invalid X-Api-Key")
	}
	if apiErr.ReqID != "req-nested-1" {
		t.Fatalf("reqid = %q, want %q", apiErr.ReqID, "req-nested-1")
	}
	if apiErr.TraceID != "trace-nested-1" {
		t.Fatalf("trace_id = %q, want %q", apiErr.TraceID, "trace-nested-1")
	}
	if apiErr.LogID != "log-nested-1" {
		t.Fatalf("logid = %q, want %q", apiErr.LogID, "log-nested-1")
	}
}

func TestParseAPIErrorTopLevelMetadataAliases(t *testing.T) {
	err := parseAPIError(http.StatusBadRequest, []byte(`{"status_code":45000001,"error":"bad request","request_id":"req-top","trace_id":"trace-top","logid":"log-top","connect_id":"conn-top"}`), "log-header")
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}

	if apiErr.Code != 45000001 {
		t.Fatalf("code = %d, want 45000001", apiErr.Code)
	}
	if apiErr.Message != "bad request" {
		t.Fatalf("message = %q, want bad request", apiErr.Message)
	}
	if apiErr.ReqID != "req-top" || apiErr.TraceID != "trace-top" || apiErr.LogID != "log-top" || apiErr.ConnectID != "conn-top" {
		t.Fatalf("metadata = reqid %q trace %q log %q conn %q", apiErr.ReqID, apiErr.TraceID, apiErr.LogID, apiErr.ConnectID)
	}
}

func TestParseAPIErrorMergesNestedHeaderFallback(t *testing.T) {
	err := parseAPIError(http.StatusBadRequest, []byte(`{"code":45000001,"header":{"message":"nested message","request_id":"req-header","trace_id":"trace-header","logid":"log-header"}}`), "")
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != 45000001 || apiErr.Message != "nested message" {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.ReqID != "req-header" || apiErr.TraceID != "trace-header" || apiErr.LogID != "log-header" {
		t.Fatalf("metadata = reqid %q trace %q log %q", apiErr.ReqID, apiErr.TraceID, apiErr.LogID)
	}
}

func TestParseAPIErrorEmptyBody(t *testing.T) {
	err := parseAPIError(http.StatusTooManyRequests, nil, "log-empty")
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != http.StatusTooManyRequests || apiErr.Message != http.StatusText(http.StatusTooManyRequests) {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.LogID != "log-empty" {
		t.Fatalf("logid = %q, want log-empty", apiErr.LogID)
	}
}

func TestParseAPIErrorNonJSONBody(t *testing.T) {
	err := parseAPIError(http.StatusBadGateway, []byte("upstream failed"), "log-text")
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != http.StatusBadGateway || apiErr.Message != "upstream failed" {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.LogID != "log-text" {
		t.Fatalf("logid = %q, want log-text", apiErr.LogID)
	}
}

func TestParseWSErrorPayloadNestedHeader(t *testing.T) {
	err := parseWSErrorPayload([]byte(`{"header":{"status_code":55000001,"message":"session failed","request_id":"req-ws","logid":"log-ws"}}`), 0)
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != 55000001 || apiErr.Message != "session failed" {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.ReqID != "req-ws" || apiErr.LogID != "log-ws" {
		t.Fatalf("metadata = reqid %q logid %q", apiErr.ReqID, apiErr.LogID)
	}
}
