package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

func TestChunkAudio(t *testing.T) {
	chunks := chunkAudio([]byte{1, 2, 3, 4, 5}, 2)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	if !bytes.Equal(chunks[0], []byte{1, 2}) || !bytes.Equal(chunks[2], []byte{5}) {
		t.Fatalf("chunks = %#v", chunks)
	}
}

func TestDemoToolOutput(t *testing.T) {
	output := demoToolOutput(doubaospeech.RealtimeDuplexFunctionCall{
		Name:      "lookup_weather",
		Arguments: `{"city":"深圳"}`,
	})

	var body map[string]any
	if err := json.Unmarshal([]byte(output), &body); err != nil {
		t.Fatalf("tool output is not json: %v", err)
	}
	if body["city"] != "深圳" {
		t.Fatalf("city = %v", body["city"])
	}
	if !strings.Contains(body["summary"].(string), "语音对话测试") {
		t.Fatalf("summary = %v", body["summary"])
	}
}

func TestCompactForPrompt(t *testing.T) {
	got := compactForPrompt("  hello\n\nworld\tagain  ")
	if got != "hello world again" {
		t.Fatalf("compact = %q", got)
	}
	long := compactForPrompt(strings.Repeat("字", 100))
	if len([]rune(long)) != 80 {
		t.Fatalf("long compact rune length = %d, want 80", len([]rune(long)))
	}
}

func TestNextOldPrompt(t *testing.T) {
	got := nextOldPrompt(1, "old text", "duplex text")
	if !strings.Contains(got, "old text") || !strings.Contains(got, "duplex text") || !strings.Contains(got, "第 2 轮") {
		t.Fatalf("prompt = %q", got)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := exampleConfig{
		Rounds:      1,
		AppID:       "app",
		RealtimeKey: "rt",
		DuplexKey:   "duplex",
		ASRKey:      "asr",
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("validateConfig valid error = %v", err)
	}

	missingDuplex := valid
	missingDuplex.DuplexKey = ""
	if err := validateConfig(missingDuplex); err == nil {
		t.Fatalf("validateConfig expected missing duplex error")
	}
}

func TestLiveRealtimeDuplex(t *testing.T) {
	if os.Getenv("DOUBAO_RUN_LIVE") != "1" {
		t.Skip("set DOUBAO_RUN_LIVE=1 to run live realtime duplex conversation smoke")
	}
	if err := run([]string{"-rounds", "2"}); err != nil {
		t.Fatalf("live realtime duplex conversation error = %v", err)
	}
}
