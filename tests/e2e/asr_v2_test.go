package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

const (
	asrV2AudioPath     = "../../examples/asr_v2_sauc_ws/sample_zh_16k.pcm"
	asrV2ChunkDuration = 100 * time.Millisecond
	asrV2ChunkSize     = 3200
	asrV2VADDeadline   = 4 * time.Second
)

// TestASRV2VADEndpointing verifies that BigASR accepts the typed VAD request
// and emits a definite utterance after streamed silence without an explicit
// final audio packet. It is opt-in because it uses paid credentials.
func TestASRV2VADEndpointing(t *testing.T) {
	loadE2EEnv(t)
	if os.Getenv("DOUBAO_RUN_LIVE") != "1" {
		t.Skip("set DOUBAO_RUN_LIVE=1 in tests/e2e/.env to run the ASR E2E test")
	}

	appID := firstEnv("DOUBAO_ASR_APP_ID", "DOUBAO_APP_ID", "DOUBAO_REALTIME_APP_ID")
	if appID == "" {
		t.Fatal("DOUBAO_ASR_APP_ID, DOUBAO_APP_ID, or DOUBAO_REALTIME_APP_ID is required")
	}
	apiKey := firstEnv("DOUBAO_ASR_API_KEY", "DOUBAO_API_KEY", "DOUBAO_REALTIME_API_KEY")
	if apiKey == "" {
		t.Fatal("DOUBAO_ASR_API_KEY, DOUBAO_API_KEY, or DOUBAO_REALTIME_API_KEY is required")
	}
	resourceID := firstEnv("DOUBAO_ASR_RESOURCE_ID")
	if resourceID == "" {
		resourceID = doubaospeech.ResourceASRStream
	}

	audio, err := os.ReadFile(asrV2AudioPath)
	if err != nil {
		t.Fatalf("read ASR E2E fixture: %v", err)
	}
	if bytes.HasPrefix(bytes.TrimSpace(audio), []byte("version https://git-lfs.github.com/spec/v1")) {
		t.Fatal("ASR E2E fixture is a Git LFS pointer; run git lfs pull")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endWindowSize := 800
	forceToSpeechTime := 0
	vadSegmentDuration := 3000
	falseValue := false
	client := doubaospeech.NewClient(
		appID,
		doubaospeech.WithAPIKey(apiKey),
		doubaospeech.WithResourceID(resourceID),
		doubaospeech.WithUserID("asr-v2-e2e"),
	)
	session, err := client.ASRV2.OpenStreamSession(ctx, &doubaospeech.ASRV2Config{
		Format:     doubaospeech.FormatPCM,
		SampleRate: doubaospeech.SampleRate16000,
		Request: &doubaospeech.ASRV2RequestConfig{
			EnableITN:          &falseValue,
			EnablePunc:         &falseValue,
			ResultType:         "full",
			VADSegmentDuration: &vadSegmentDuration,
			EndWindowSize:      &endWindowSize,
			ForceToSpeechTime:  &forceToSpeechTime,
		},
	})
	if err != nil {
		t.Fatalf("open ASR E2E session: %v", err)
	}
	defer session.Close()

	finals := make(chan asrFinal, 16)
	recvErrors := make(chan error, 1)
	go receiveASRFinals(session, finals, recvErrors)

	for offset := 0; offset < len(audio); offset += asrV2ChunkSize {
		end := min(offset+asrV2ChunkSize, len(audio))
		if err := session.SendAudio(ctx, audio[offset:end], false); err != nil {
			t.Fatalf("send ASR E2E speech audio: %v", err)
		}
		waitInterval(t, ctx, asrV2ChunkDuration)
	}
	drainASRFinals(finals)

	speechEndedAt := time.Now()
	silence := make([]byte, asrV2ChunkSize)
	deadline := time.NewTimer(asrV2VADDeadline)
	defer deadline.Stop()
	ticker := time.NewTicker(asrV2ChunkDuration)
	defer ticker.Stop()

	for {
		select {
		case final := <-finals:
			if final.receivedAt.Before(speechEndedAt) {
				continue
			}
			latency := final.receivedAt.Sub(speechEndedAt)
			if strings.TrimSpace(final.text) == "" {
				t.Fatal("BigASR returned a definite utterance without transcript text")
			}
			if latency > asrV2VADDeadline {
				t.Fatalf("BigASR VAD final latency = %s, want <= %s", latency, asrV2VADDeadline)
			}
			t.Logf("BigASR VAD final latency=%s transcript=%q resource=%s", latency, final.text, resourceID)
			if err := session.SendAudio(ctx, nil, true); err != nil {
				t.Fatalf("finish ASR E2E session: %v", err)
			}
			return
		case err := <-recvErrors:
			t.Fatalf("receive ASR E2E result: %v", err)
		case <-ticker.C:
			if err := session.SendAudio(ctx, silence, false); err != nil {
				t.Fatalf("send ASR E2E silence: %v", err)
			}
		case <-deadline.C:
			t.Fatalf("BigASR did not emit a definite utterance within %s of streamed silence without audio EOS", asrV2VADDeadline)
		case <-ctx.Done():
			t.Fatalf("ASR E2E context ended: %v", ctx.Err())
		}
	}
}

type asrFinal struct {
	text       string
	receivedAt time.Time
}

func receiveASRFinals(session *doubaospeech.ASRV2Session, finals chan<- asrFinal, recvErrors chan<- error) {
	for result, err := range session.Recv() {
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
				recvErrors <- err
			}
			return
		}
		if result == nil || !result.IsFinal {
			continue
		}
		text := strings.TrimSpace(result.Text)
		if text == "" {
			for _, utterance := range result.Utterances {
				if utterance.Definite && strings.TrimSpace(utterance.Text) != "" {
					text = strings.TrimSpace(utterance.Text)
				}
			}
		}
		finals <- asrFinal{text: text, receivedAt: time.Now()}
	}
}

func drainASRFinals(finals <-chan asrFinal) {
	for {
		select {
		case <-finals:
		default:
			return
		}
	}
}

func waitInterval(t *testing.T, ctx context.Context, interval time.Duration) {
	t.Helper()
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatalf("ASR E2E context ended while pacing audio: %v", ctx.Err())
	}
}

func loadE2EEnv(t *testing.T) {
	t.Helper()
	for _, path := range []string{".env", filepath.Join("..", "..", ".env")} {
		loaded, err := loadEnvFile(t, path)
		if err != nil {
			t.Fatalf("load E2E environment file %s: %v", path, err)
		}
		if loaded {
			return
		}
	}
}

func loadEnvFile(t *testing.T, path string) (bool, error) {
	t.Helper()
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return false, fmt.Errorf("line %d must be NAME=VALUE", lineNumber)
		}
		value, err = parseEnvValue(strings.TrimSpace(value))
		if err != nil {
			return false, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if _, exists := os.LookupEnv(name); !exists {
			t.Setenv(name, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return true, nil
}

func parseEnvValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted value")
		}
		return unquoted, nil
	}
	return value, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
