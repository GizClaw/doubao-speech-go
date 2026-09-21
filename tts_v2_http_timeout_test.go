package doubaospeech

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// These use real HTTP so both Client.Do and streamed-body cancellation are
// exercised. Run the slow cases concurrently to keep their wall time bounded.
func TestTTSV2HTTPStreamBeyondDefaultTimeout(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if phase == "body" {
					_, _ = io.WriteString(w, "{\"code\":0,\"data\":\"Zmlyc3Q=\"}\n")
					w.(http.Flusher).Flush()
				}
				select {
				case <-time.After(31 * time.Second):
				case <-r.Context().Done():
					return
				}
				_, _ = io.WriteString(w, "{\"code\":0,\"data\":\"bGFzdA==\"}\n{\"code\":20000000}\n")
			}))
			defer server.Close()
			client := NewClient("test", WithBaseURL(server.URL))
			chunks, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(t.Context(), timeoutTestRequest()))
			if err != nil {
				t.Fatalf("stream must survive the former 30s default during %s: %v", phase, err)
			}
			var audio strings.Builder
			for _, chunk := range chunks {
				audio.Write(chunk.Audio)
			}
			want := "last"
			if phase == "body" {
				want = "firstlast"
			}
			if audio.String() != want || len(chunks) == 0 || !chunks[len(chunks)-1].IsLast {
				t.Fatalf("audio = %q, chunks = %+v; want %q and final frame", audio.String(), chunks, want)
			}
		})
	}
}

func TestTTSV2HTTPStreamSlowHeadersCallerDeadline(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 32*time.Second)
	defer cancel()
	client := NewClient("test", WithBaseURL(server.URL))
	_, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(ctx, timeoutTestRequest()))
	if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("request error = %v, caller context = %v; want caller deadline", err, ctx.Err())
	}
}

func TestTTSV2HTTPStreamCallerCancellation(t *testing.T) {
	for _, phase := range []string{"headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			received := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if phase == "body" {
					_, _ = io.WriteString(w, "{\"code\":0,\"data\":\"Zmlyc3Q=\"}\n")
					w.(http.Flusher).Flush()
				}
				close(received)
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if phase == "headers" {
				go func() {
					<-received
					cancel()
				}()
			}
			client := NewClient("test", WithBaseURL(server.URL))
			var streamErr error
			for chunk, err := range client.TTSV2.Stream(ctx, timeoutTestRequest()) {
				if err != nil {
					streamErr = err
					break
				}
				if phase == "body" && len(chunk.Audio) > 0 {
					cancel()
				}
			}
			if !errors.Is(streamErr, context.Canceled) {
				t.Fatalf("stream error = %v, want caller cancellation during %s", streamErr, phase)
			}
		})
	}
}

func timeoutTestRequest() *TTSV2Request {
	return &TTSV2Request{Text: "Timeout policy test.", Speaker: "test-speaker"}
}
