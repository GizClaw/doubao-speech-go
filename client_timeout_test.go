package doubaospeech

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPTimeoutPolicy(t *testing.T) {
	var typedNil *panicOnNilDoer
	for _, test := range []struct {
		name          string
		opts          []Option
		unary, stream time.Duration
	}{
		{name: "default", unary: 30 * time.Second},
		{name: "explicit", opts: []Option{WithTimeout(time.Second)}, unary: time.Second, stream: time.Second},
		{name: "disabled", opts: []Option{WithTimeout(0)}},
		{name: "nil client", opts: []Option{WithHTTPClient(nil)}, unary: 30 * time.Second},
		{name: "typed nil doer", opts: []Option{WithHTTPTransport(typedNil)}, unary: 30 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := NewClient("test", test.opts...)
			if got := client.config.httpClient.(*http.Client).Timeout; got != test.unary {
				t.Errorf("non-streaming timeout = %v, want %v", got, test.unary)
			}
			if got := client.config.streamHTTPClient.(*http.Client).Timeout; got != test.stream {
				t.Errorf("streaming timeout = %v, want %v", got, test.stream)
			}
		})
	}
}

func TestCustomHTTPClientTimeoutPrecedence(t *testing.T) {
	custom := &http.Client{Timeout: 7 * time.Second}
	doer := &panicOnNilDoer{}
	for _, opts := range [][]Option{
		{WithHTTPClient(custom), WithTimeout(time.Second)},
		{WithTimeout(time.Second), WithHTTPClient(custom)},
		{WithHTTPTransport(custom), WithTimeout(time.Second)},
		{WithTimeout(time.Second), WithHTTPTransport(custom)},
		{WithHTTPTransport(doer), WithTimeout(time.Second)},
		{WithTimeout(time.Second), WithHTTPTransport(doer)},
	} {
		client := NewClient("test", opts...)
		if client.config.httpClient != custom && client.config.httpClient != doer {
			t.Fatal("custom transport was replaced")
		}
		if client.config.streamHTTPClient != client.config.httpClient {
			t.Fatal("streaming must use the same supplied transport")
		}
	}
	if custom.Timeout != 7*time.Second {
		t.Fatalf("custom client was mutated: timeout = %v", custom.Timeout)
	}
}

func TestTTSV2HTTPStreamExplicitTimeout(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "option", true: "custom client"}[custom], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				<-r.Context().Done()
			}))
			defer server.Close()
			opts := []Option{WithBaseURL(server.URL), WithTimeout(50 * time.Millisecond)}
			if custom {
				opts = []Option{WithBaseURL(server.URL), WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}), WithTimeout(time.Hour)}
			}
			client := NewClient("test", opts...)
			_, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(t.Context(), timeoutTestRequest()))
			if !errors.Is(err, context.DeadlineExceeded) || t.Context().Err() != nil {
				t.Fatalf("error = %v, caller context = %v; want explicit client timeout", err, t.Context().Err())
			}
		})
	}
}
