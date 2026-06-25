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
		model      string
		prompt     string
		speaker    string
		audioURL   string
		imageURL   string
		format     string
		sampleRate int
		outputPath string
		timeoutSec int
	)

	flag.StringVar(&model, "model", doubaospeech.ModelSeedAudio10, "Audio Generation model ID")
	flag.StringVar(&prompt, "prompt", "Generate a short cinematic notification sound.", "Prompt or text content")
	flag.StringVar(&speaker, "speaker", "", "Optional speaker or cloned voice ID reference")
	flag.StringVar(&audioURL, "audio-url", "", "Optional reference audio URL")
	flag.StringVar(&imageURL, "image-url", "", "Optional reference image URL")
	flag.StringVar(&format, "format", "wav", "Output audio format: wav/mp3/pcm/ogg_opus")
	flag.IntVar(&sampleRate, "sample-rate", 24000, "Output audio sample rate")
	flag.StringVar(&outputPath, "output", "audio_generation_output.wav", "Output audio file path")
	flag.IntVar(&timeoutSec, "timeout-sec", 180, "Request timeout in seconds")
	flag.Parse()

	appID := strings.TrimSpace(os.Getenv("DOUBAO_APP_ID"))
	apiKey := strings.TrimSpace(os.Getenv("DOUBAO_API_KEY"))
	if appID == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "missing DOUBAO_APP_ID or DOUBAO_API_KEY")
		os.Exit(2)
	}

	model = strings.TrimSpace(model)
	prompt = strings.TrimSpace(prompt)
	if model == "" {
		fmt.Fprintln(os.Stderr, "-model cannot be empty")
		os.Exit(2)
	}
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "-prompt cannot be empty")
		os.Exit(2)
	}
	if timeoutSec <= 0 {
		timeoutSec = 180
	}

	client := doubaospeech.NewClient(appID,
		doubaospeech.WithAPIKey(apiKey),
	)

	req := &doubaospeech.AudioGenerationCreateRequest{
		Model:      model,
		TextPrompt: prompt,
		AudioConfig: &doubaospeech.AudioGenerationAudioConfig{
			Format:     doubaospeech.AudioFormat(strings.ToLower(strings.TrimSpace(format))),
			SampleRate: doubaospeech.SampleRate(sampleRate),
		},
	}

	reference, hasReference := buildReference(speaker, audioURL, imageURL)
	if hasReference {
		req.References = []doubaospeech.AudioGenerationReference{reference}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	resp, err := client.AudioGeneration.Create(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audio generation failed: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Audio) > 0 {
		if err := ensureOutputDir(outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "prepare output directory failed: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(outputPath, resp.Audio, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write output file failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("audio generation finished: bytes=%d duration=%.3fs original_duration=%.3fs reqid=%s logid=%s url=%s output=%s\n",
		len(resp.Audio),
		resp.Duration,
		resp.OriginalDuration,
		resp.ReqID,
		resp.LogID,
		resp.URL,
		outputPath,
	)
}

func buildReference(speaker, audioURL, imageURL string) (doubaospeech.AudioGenerationReference, bool) {
	speaker = strings.TrimSpace(speaker)
	audioURL = strings.TrimSpace(audioURL)
	imageURL = strings.TrimSpace(imageURL)

	if speaker == "" && audioURL == "" && imageURL == "" {
		return doubaospeech.AudioGenerationReference{}, false
	}

	return doubaospeech.AudioGenerationReference{
		Speaker:  speaker,
		AudioURL: audioURL,
		ImageURL: imageURL,
	}, true
}

func ensureOutputDir(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
