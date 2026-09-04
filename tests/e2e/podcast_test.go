package e2e_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

// TestPodcastGeneratesAudio exercises the live PodcastTTS WebSocket lifecycle.
// It is opt-in because it generates paid provider content. Audio stays in
// memory so the test does not consume persistent disk space.
func TestPodcastGeneratesAudio(t *testing.T) {
	loadE2EEnv(t)
	if os.Getenv("DOUBAO_RUN_LIVE") != "1" {
		t.Skip("set DOUBAO_RUN_LIVE=1 to run the Podcast E2E test")
	}

	apiKey := firstEnv("DOUBAO_PODCAST_API_KEY", "MODEL_SPEECH_API_KEY", "DOUBAO_API_KEY", "DOUBAO_REALTIME_API_KEY")
	if apiKey == "" {
		t.Fatal("DOUBAO_PODCAST_API_KEY, MODEL_SPEECH_API_KEY, DOUBAO_API_KEY, or DOUBAO_REALTIME_API_KEY is required")
	}
	appID := firstEnv("DOUBAO_PODCAST_APP_ID", "DOUBAO_APP_ID", "DOUBAO_REALTIME_APP_ID")
	resourceID := firstEnv("DOUBAO_PODCAST_RESOURCE_ID")
	if resourceID == "" {
		resourceID = doubaospeech.ResourcePodcast
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := doubaospeech.NewClient(
		appID,
		doubaospeech.WithAPIKey(apiKey),
		doubaospeech.WithResourceID(resourceID),
	)
	headMusic := false
	tailMusic := false
	session, err := client.Podcast.OpenSession(ctx, &doubaospeech.PodcastRequest{
		InputID:      fmt.Sprintf("doubao-speech-go-e2e-%d", time.Now().UnixNano()),
		PromptText:   "请用两位主持人的简短中文对话解释月亮为什么会有圆缺，两到三轮即可。",
		UseHeadMusic: &headMusic,
		UseTailMusic: &tailMusic,
		AudioConfig: &doubaospeech.PodcastAudioConfig{
			Format:     doubaospeech.FormatMP3,
			SampleRate: doubaospeech.SampleRate24000,
		},
	})
	if err != nil {
		t.Fatalf("open Podcast E2E session: %v", err)
	}
	defer session.Close()
	if session.TaskID() == "" {
		t.Fatal("Podcast E2E session returned an empty task ID")
	}

	var audioBytes int
	var rounds int
	for {
		event, err := session.RecvEvent(ctx)
		if err != nil {
			t.Fatalf("receive Podcast E2E event: %v", err)
		}
		switch event.Type {
		case doubaospeech.PodcastEventAudio:
			audioBytes += len(event.Audio)
		case doubaospeech.PodcastEventRoundFinished:
			if event.IsError {
				t.Fatalf("Podcast E2E round %d reported an error", event.RoundID)
			}
			rounds++
		case doubaospeech.PodcastEventSessionFinished:
			if rounds == 0 {
				t.Fatal("Podcast E2E completed without a finished round")
			}
			if audioBytes == 0 {
				t.Fatal("Podcast E2E completed without audio")
			}
			t.Logf("Podcast generated task_id=%s rounds=%d audio_bytes=%d resource=%s", session.TaskID(), rounds, audioBytes, resourceID)
			return
		}
	}
}
