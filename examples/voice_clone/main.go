package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

func main() {
	var (
		speakerID    string
		audioPath    string
		resourceID   string
		modelType    int
		timeoutSec   int
		pollInterval int
	)

	flag.StringVar(&speakerID, "speaker-id", "", "Voice clone speaker ID")
	flag.StringVar(&audioPath, "audio", "", "Audio file path for voice clone training")
	flag.StringVar(&resourceID, "resource-id", "", "Voice clone resource ID (optional: seed-icl-1.0 or seed-icl-2.0; empty means auto by model-type)")
	flag.IntVar(&modelType, "model-type", 1, "Voice clone model type")
	flag.IntVar(&timeoutSec, "timeout-sec", 180, "Task wait timeout in seconds")
	flag.IntVar(&pollInterval, "poll-interval-ms", 2000, "Task polling interval in milliseconds")
	flag.Parse()

	appID := strings.TrimSpace(os.Getenv("DOUBAO_APP_ID"))
	apiKey := strings.TrimSpace(os.Getenv("DOUBAO_API_KEY"))
	if appID == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "missing DOUBAO_APP_ID or DOUBAO_API_KEY")
		os.Exit(2)
	}

	speakerID = strings.TrimSpace(speakerID)
	if speakerID == "" {
		fmt.Fprintln(os.Stderr, "-speaker-id cannot be empty")
		os.Exit(2)
	}

	audioPath = strings.TrimSpace(audioPath)
	if audioPath == "" {
		fmt.Fprintln(os.Stderr, "-audio cannot be empty")
		os.Exit(2)
	}

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read audio file failed: %v\n", err)
		os.Exit(1)
	}
	if len(audioData) == 0 {
		fmt.Fprintln(os.Stderr, "audio file is empty")
		os.Exit(2)
	}

	resourceID = strings.TrimSpace(resourceID)
	opts := []doubaospeech.Option{
		doubaospeech.WithUserID("voice-clone-example"),
		doubaospeech.WithAPIKey(apiKey),
	}
	if resourceID != "" {
		opts = append(opts, doubaospeech.WithResourceID(resourceID))
	}

	if timeoutSec <= 0 {
		timeoutSec = 180
	}
	if pollInterval <= 0 {
		pollInterval = 2000
	}

	client := doubaospeech.NewClient(appID, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	task, err := client.VoiceClone.Upload(ctx, &doubaospeech.VoiceCloneRequest{
		VoiceID:       speakerID,
		Audio:         audioData,
		AudioFileName: filepath.Base(audioPath),
		ModelType:     modelType,
		ResourceID:    resourceID,
		PollInterval:  time.Duration(pollInterval) * time.Millisecond,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "submit voice clone task failed: %v\n", err)
		os.Exit(1)
	}

	status, err := task.Wait(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wait voice clone task failed: %v\n", err)
		os.Exit(1)
	}

	if status == nil {
		fmt.Fprintln(os.Stderr, "voice clone task finished with empty result")
		os.Exit(1)
	}

	fmt.Printf(
		"voice clone completed: speaker_id=%s status=%s version=%s demo_audio=%s\n",
		status.SpeakerID,
		status.Status,
		status.Version,
		status.DemoAudio,
	)
}
