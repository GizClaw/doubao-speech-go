package doubaospeech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTTSV2HTTPStreamChunkSequenceAndFinalFrame(t *testing.T) {
	type capturedRequest struct {
		Method     string
		Path       string
		AppID      string
		APIKey     string
		ResourceID string
		Body       ttsV2HTTPStreamRequest
	}

	requestCh := make(chan capturedRequest, 1)

	firstAudio := []byte("chunk-audio-1")
	secondAudio := []byte("chunk-audio-2")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ttsV2HTTPStreamRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		requestCh <- capturedRequest{
			Method:     r.Method,
			Path:       r.URL.Path,
			AppID:      r.Header.Get("X-Api-App-Id"),
			APIKey:     r.Header.Get("X-Api-Key"),
			ResourceID: r.Header.Get("X-Api-Resource-Id"),
			Body:       body,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Tt-Logid", "log-stream-1")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "flush is not supported", http.StatusInternalServerError)
			return
		}

		_, _ = fmt.Fprintf(
			w,
			`{"reqid":"req-stream-1","trace_id":"trace-stream-1","code":0,"message":"","data":"%s"}`+"\n",
			base64.StdEncoding.EncodeToString(firstAudio),
		)
		flusher.Flush()

		_, _ = fmt.Fprintf(
			w,
			`{"reqid":"req-stream-1","code":0,"message":"","data":"%s"}`+"\n",
			base64.StdEncoding.EncodeToString(secondAudio),
		)
		flusher.Flush()

		_, _ = fmt.Fprintln(w, `{"reqid":"req-stream-1","code":20000000,"message":"ok","data":null}`)
		flusher.Flush()
	}))
	defer server.Close()

	client := NewClient("app-test",
		WithAPIKey("key-test"),
		WithBaseURL(server.URL),
		WithUserID("stream-user"),
	)

	chunks, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(context.Background(), &TTSV2Request{
		Text:       "hello stream",
		Speaker:    "zh_female_xiaohe_uranus_bigtts",
		Format:     FormatPCM,
		SampleRate: SampleRate16000,
		BitRate:    64000,
		SpeechRate: 10,
		PitchRate:  -5,
		VolumeRate: 8,
		ResourceID: ResourceTTSV2,
	}))
	if err != nil {
		t.Fatalf("Stream error = %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("chunk count = %d, want 3", len(chunks))
	}

	combinedAudio := bytes.Join([][]byte{chunks[0].Audio, chunks[1].Audio}, nil)
	if !bytes.Equal(combinedAudio, append(firstAudio, secondAudio...)) {
		t.Fatalf("unexpected combined audio: got %q", string(combinedAudio))
	}

	if chunks[0].IsLast {
		t.Fatalf("first chunk should not be final")
	}
	if chunks[0].ReqID != "req-stream-1" {
		t.Fatalf("first chunk reqid = %q, want %q", chunks[0].ReqID, "req-stream-1")
	}
	if chunks[0].TraceID != "trace-stream-1" {
		t.Fatalf("first chunk trace_id = %q, want %q", chunks[0].TraceID, "trace-stream-1")
	}
	if chunks[0].LogID != "log-stream-1" {
		t.Fatalf("first chunk log_id = %q, want %q", chunks[0].LogID, "log-stream-1")
	}
	if chunks[1].IsLast {
		t.Fatalf("second chunk should not be final")
	}
	if !chunks[2].IsLast {
		t.Fatalf("last chunk should be final")
	}
	if len(chunks[2].Audio) != 0 {
		t.Fatalf("final chunk audio len = %d, want 0", len(chunks[2].Audio))
	}

	captured := <-requestCh
	if captured.Method != http.MethodPost {
		t.Fatalf("method = %s, want %s", captured.Method, http.MethodPost)
	}
	if captured.Path != ttsV2HTTPStreamPath {
		t.Fatalf("path = %s, want %s", captured.Path, ttsV2HTTPStreamPath)
	}
	if captured.AppID != "app-test" {
		t.Fatalf("X-Api-App-Id = %q, want %q", captured.AppID, "app-test")
	}
	if captured.APIKey != "key-test" {
		t.Fatalf("X-Api-Key = %q, want %q", captured.APIKey, "key-test")
	}
	if captured.ResourceID != ResourceTTSV2 {
		t.Fatalf("X-Api-Resource-Id = %q, want %q", captured.ResourceID, ResourceTTSV2)
	}

	if captured.Body.User.UID != "stream-user" {
		t.Fatalf("user.uid = %q, want %q", captured.Body.User.UID, "stream-user")
	}
	if captured.Body.ReqParams.Text != "hello stream" {
		t.Fatalf("req_params.text = %q, want %q", captured.Body.ReqParams.Text, "hello stream")
	}
	if captured.Body.ReqParams.Speaker != "zh_female_xiaohe_uranus_bigtts" {
		t.Fatalf("req_params.speaker = %q", captured.Body.ReqParams.Speaker)
	}
	if captured.Body.ReqParams.AudioParams.Format != string(FormatPCM) {
		t.Fatalf("audio_params.format = %q, want %q", captured.Body.ReqParams.AudioParams.Format, FormatPCM)
	}
	if captured.Body.ReqParams.AudioParams.SampleRate != int(SampleRate16000) {
		t.Fatalf("audio_params.sample_rate = %d, want %d", captured.Body.ReqParams.AudioParams.SampleRate, SampleRate16000)
	}
	if captured.Body.ReqParams.AudioParams.BitRate != 64000 {
		t.Fatalf("audio_params.bit_rate = %d, want 64000", captured.Body.ReqParams.AudioParams.BitRate)
	}
	if captured.Body.ReqParams.AudioParams.SpeechRate != 10 {
		t.Fatalf("audio_params.speech_rate = %d, want 10", captured.Body.ReqParams.AudioParams.SpeechRate)
	}
	if captured.Body.ReqParams.AudioParams.PitchRate != -5 {
		t.Fatalf("audio_params.pitch_rate = %d, want -5", captured.Body.ReqParams.AudioParams.PitchRate)
	}
	if captured.Body.ReqParams.AudioParams.VolumeRate != 8 {
		t.Fatalf("audio_params.volume_rate = %d, want 8", captured.Body.ReqParams.AudioParams.VolumeRate)
	}
}

func TestTTSV2HTTPStreamOnlyFinalFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, `{"reqid":"req-final-only","code":20000000,"message":"ok","data":null}`)
	}))
	defer server.Close()

	client := NewClient("app-test",
		WithAPIKey("key-test"),
		WithBaseURL(server.URL),
	)

	chunks, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(context.Background(), &TTSV2Request{
		Text:    "final only",
		Speaker: "zh_female_vv_uranus_bigtts",
	}))
	if err != nil {
		t.Fatalf("Stream error = %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("chunk count = %d, want 1", len(chunks))
	}
	if !chunks[0].IsLast {
		t.Fatalf("single chunk should be final")
	}
	if len(chunks[0].Audio) != 0 {
		t.Fatalf("single final chunk audio len = %d, want 0", len(chunks[0].Audio))
	}
}

func TestTTSV2HTTPStreamResourceSpeakerMismatchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, `{"reqid":"req-mismatch","code":55000000,"message":"resource ID is mismatched with speaker related resource"}`)
	}))
	defer server.Close()

	client := NewClient("app-test",
		WithAPIKey("key-test"),
		WithBaseURL(server.URL),
	)

	_, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(context.Background(), &TTSV2Request{
		Text:       "mismatch test",
		Speaker:    "zh_female_shuangkuaisisi_moon_bigtts",
		ResourceID: ResourceTTSV2,
	}))
	if err == nil {
		t.Fatalf("expected mismatch error")
	}

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != 55000000 {
		t.Fatalf("error code = %d, want 55000000", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "resource ID is mismatched with speaker related resource") {
		t.Fatalf("unexpected error message = %q", apiErr.Message)
	}
	if apiErr.ReqID != "req-mismatch" {
		t.Fatalf("reqid = %q, want %q", apiErr.ReqID, "req-mismatch")
	}
}

func TestTTSV2HTTPStreamErrorNestedHeaderMetadata(t *testing.T) {
	line := []byte(`{"code":55000000,"message":"failed","header":{"request_id":"req-header","trace_id":"trace-header","logid":"log-header"}}`)

	_, _, _, err := parseTTSV2HTTPStreamLine(line, responseMetadata{})
	if err == nil {
		t.Fatalf("expected business error")
	}
	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != 55000000 || apiErr.Message != "failed" {
		t.Fatalf("error = %#v", apiErr)
	}
	if apiErr.ReqID != "req-header" || apiErr.TraceID != "trace-header" || apiErr.LogID != "log-header" {
		t.Fatalf("metadata = reqid %q trace %q log %q", apiErr.ReqID, apiErr.TraceID, apiErr.LogID)
	}
}

func TestTTSV2HTTPStreamEOFWithoutFinalFrameReturnsError(t *testing.T) {
	partialAudio := []byte("partial-audio")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(
			w,
			`{"reqid":"req-partial","code":0,"message":"","data":"%s"}`+"\n",
			base64.StdEncoding.EncodeToString(partialAudio),
		)
	}))
	defer server.Close()

	client := NewClient("app-test",
		WithAPIKey("key-test"),
		WithBaseURL(server.URL),
	)

	chunks, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(context.Background(), &TTSV2Request{
		Text:    "eof without final",
		Speaker: "zh_female_vv_uranus_bigtts",
	}))
	if err == nil {
		t.Fatalf("expected error when stream ended without final frame")
	}

	if len(chunks) != 1 {
		t.Fatalf("chunk count = %d, want 1", len(chunks))
	}
	if !bytes.Equal(chunks[0].Audio, partialAudio) {
		t.Fatalf("unexpected audio chunk = %q", string(chunks[0].Audio))
	}
	if chunks[0].IsLast {
		t.Fatalf("partial chunk should not be final")
	}

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if apiErr.Code != CodeServerError {
		t.Fatalf("error code = %d, want %d", apiErr.Code, CodeServerError)
	}
	if !strings.Contains(apiErr.Message, "ended before final frame") {
		t.Fatalf("unexpected error message = %q", apiErr.Message)
	}
	if apiErr.ReqID != "req-partial" {
		t.Fatalf("reqid = %q, want %q", apiErr.ReqID, "req-partial")
	}
}

func TestTTSV2HTTPStreamBodyTermination(t *testing.T) {
	audioLine := `{"reqid":"req-termination","trace_id":"trace-termination","log_id":"log-frame","code":0,"data":"YXVkaW8="}`
	finalLine := `{"code":20000000,"data":null}`
	partialLine := `{"reqid":"req-incomplete","code":0,"data":"YXV`

	tests := []struct {
		name        string
		body        string
		truncated   bool
		wantChunks  int
		wantFinal   bool
		wantCode    int
		wantMessage string
	}{
		{
			name: "truncated mid-line", body: audioLine + "\n" + partialLine, truncated: true,
			wantChunks: 1, wantCode: CodeServerError, wantMessage: "tts stream truncated before final frame",
		},
		{
			name: "truncated between lines", body: audioLine + "\n", truncated: true,
			wantChunks: 1, wantCode: CodeServerError, wantMessage: "tts stream truncated before final frame",
		},
		{
			name: "truncated complete line without newline", body: audioLine, truncated: true,
			wantChunks: 1, wantCode: CodeServerError, wantMessage: "tts stream truncated before final frame",
		},
		{
			name: "truncated after final", body: audioLine + "\n" + finalLine + "\n" + partialLine, truncated: true,
			wantChunks: 2, wantFinal: true,
		},
		{
			name: "truncated final without newline", body: audioLine + "\n" + finalLine, truncated: true,
			wantChunks: 2, wantFinal: true,
		},
		{
			name: "clean EOF before final", body: audioLine + "\n",
			wantChunks: 1, wantCode: CodeServerError, wantMessage: "tts stream ended before final frame",
		},
		{
			name: "clean EOF complete line without newline", body: audioLine,
			wantChunks: 1, wantCode: CodeServerError, wantMessage: "tts stream ended before final frame",
		},
		{
			name: "clean EOF after final", body: audioLine + "\n" + finalLine + "\n",
			wantChunks: 2, wantFinal: true,
		},
		{
			name: "clean EOF final without newline", body: audioLine + "\n" + finalLine,
			wantChunks: 2, wantFinal: true,
		},
		{
			name: "truncated with complete business error", truncated: true,
			body:     `{"reqid":"req-termination","trace_id":"trace-termination","log_id":"log-frame","code":55000000,"message":"speaker mismatch"}`,
			wantCode: 55000000, wantMessage: "speaker mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				contentLength := len(tt.body)
				if tt.truncated {
					// Closing before Content-Length is satisfied makes net/http return io.ErrUnexpectedEOF.
					contentLength++
				}
				w.Header().Set("Content-Length", fmt.Sprint(contentLength))
				w.Header().Set("X-Tt-Logid", "log-header")
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			client := NewClient("app-test", WithAPIKey("key-test"), WithBaseURL(server.URL))
			chunks, err := collectTTSV2HTTPStreamChunks(client.TTSV2.Stream(context.Background(), &TTSV2Request{
				Text: "body termination", Speaker: "zh_female_vv_uranus_bigtts",
			}))
			if len(chunks) != tt.wantChunks {
				t.Fatalf("chunk count = %d, want %d", len(chunks), tt.wantChunks)
			}
			if len(chunks) > 0 {
				if string(chunks[0].Audio) != "audio" || chunks[0].IsLast {
					t.Fatalf("first chunk = %#v, want non-final audio chunk", chunks[0])
				}
				if got := chunks[len(chunks)-1].IsLast; got != tt.wantFinal {
					t.Fatalf("last chunk IsLast = %t, want %t", got, tt.wantFinal)
				}
			}
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("Stream error = %v, want nil", err)
				}
				return
			}
			apiErr, ok := AsError(err)
			if !ok {
				t.Fatalf("want *Error, got %T (%v)", err, err)
			}
			if apiErr.Code != tt.wantCode || apiErr.Message != tt.wantMessage {
				t.Fatalf("error = (%d, %q), want (%d, %q)", apiErr.Code, apiErr.Message, tt.wantCode, tt.wantMessage)
			}
			if apiErr.ReqID != "req-termination" || apiErr.TraceID != "trace-termination" || apiErr.LogID != "log-frame" {
				t.Fatalf("metadata = reqid %q trace %q log %q, want req-termination trace-termination log-frame", apiErr.ReqID, apiErr.TraceID, apiErr.LogID)
			}
			if got, want := apiErr.Retryable(), tt.wantCode == CodeServerError; got != want {
				t.Fatalf("Retryable = %t, want %t", got, want)
			}
		})
	}
}

func collectTTSV2HTTPStreamChunks(seq iter.Seq2[*TTSV2Chunk, error]) ([]*TTSV2Chunk, error) {
	chunks := make([]*TTSV2Chunk, 0)
	for chunk, err := range seq {
		if err != nil {
			return chunks, err
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func TestTTSV2LanguageWireJSON(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		alias    string
		want     string
	}{
		{name: "unset"},
		{name: "legacy alias", alias: "en", want: `"{\"explicit_language\":\"en\"}"`},
		{name: "explicit wins", explicit: "ja", alias: "en", want: `"{\"explicit_language\":\"ja\"}"`},
	}
	for _, language := range []string{"zh-cn", "en", "ja", "es-mx", "id", "pt-br", "ko"} {
		cases = append(cases, struct {
			name, explicit, alias, want string
		}{name: language, explicit: language, want: `"{\"explicit_language\":\"` + language + `\"}"`})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient("test-app", WithUserID("tester"))
			req, err := normalizeTTSV2Request(&TTSV2Request{
				Text: "hello", Speaker: "speaker",
				ExplicitLanguage: tc.explicit, Language: tc.alias,
			})
			if err != nil {
				t.Fatal(err)
			}
			body, err := client.TTSV2.buildStreamRequestBody(req)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			assertTTSV2EncodedLanguageFields(t, data, tc.want)
		})
	}
}

func assertTTSV2EncodedLanguageFields(t *testing.T, data []byte, wantAdditions string) {
	t.Helper()
	var body struct {
		ReqParams map[string]json.RawMessage `json:"req_params"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	got, present := body.ReqParams["additions"]
	if wantAdditions == "" {
		if present {
			t.Fatalf("additions must be absent, got %s", got)
		}
	} else if string(got) != wantAdditions {
		t.Fatalf("additions = %s, want exact JSON %s", got, wantAdditions)
	}
	var audio map[string]json.RawMessage
	if err := json.Unmarshal(body.ReqParams["audio_params"], &audio); err != nil {
		t.Fatal(err)
	}
	if language, ok := audio["language"]; ok {
		t.Fatalf("audio_params.language must be absent, got %s", language)
	}
	if language, ok := body.ReqParams["explicit_language"]; ok {
		t.Fatalf("req_params.explicit_language must be nested inside additions, got %s", language)
	}
}
