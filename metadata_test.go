package doubaospeech

import "testing"

func TestParseResponseMetadataAliases(t *testing.T) {
	meta := parseResponseMetadata(
		[]byte(`{"request_id":"req-top","trace_id":"trace-top","logid":"log-top","connect_id":"conn-top"}`),
		responseMetadata{},
	)

	if meta.ReqID != "req-top" {
		t.Fatalf("reqid = %q, want req-top", meta.ReqID)
	}
	if meta.TraceID != "trace-top" {
		t.Fatalf("trace_id = %q, want trace-top", meta.TraceID)
	}
	if meta.LogID != "log-top" {
		t.Fatalf("logid = %q, want log-top", meta.LogID)
	}
	if meta.ConnectID != "conn-top" {
		t.Fatalf("connect_id = %q, want conn-top", meta.ConnectID)
	}
}

func TestParseResponseMetadataNestedHeaderFallback(t *testing.T) {
	meta := parseResponseMetadata(
		[]byte(`{"request_id":"req-top","header":{"reqid":"req-header","trace_id":"trace-header","log_id":"log-header"}}`),
		responseMetadata{ConnectID: "conn-fallback"},
	)

	if meta.ReqID != "req-top" {
		t.Fatalf("reqid = %q, want top-level value", meta.ReqID)
	}
	if meta.TraceID != "trace-header" {
		t.Fatalf("trace_id = %q, want nested fallback", meta.TraceID)
	}
	if meta.LogID != "log-header" {
		t.Fatalf("logid = %q, want nested fallback", meta.LogID)
	}
	if meta.ConnectID != "conn-fallback" {
		t.Fatalf("connect_id = %q, want explicit fallback", meta.ConnectID)
	}
}

func TestResponseMetadataPayloadFallbackKeepsAliasGroups(t *testing.T) {
	payload := responseMetadataPayload{
		RequestID: "req-top",
		LogIDAlt:  "log-top",
	}
	fallback := responseMetadataPayload{
		ReqID: "req-fallback",
		LogID: "log-fallback",
	}

	meta := responseMetadataFromPayload(payload.withFallback(fallback))
	if meta.ReqID != "req-top" {
		t.Fatalf("reqid = %q, want top-level request_id", meta.ReqID)
	}
	if meta.LogID != "log-top" {
		t.Fatalf("logid = %q, want top-level logid", meta.LogID)
	}
}
