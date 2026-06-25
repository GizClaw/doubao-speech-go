package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

func main() {
	var (
		audioPath    = flag.String("audio", "examples/asr_v2_sauc_ws/sample_zh_16k.pcm", "path to 16 kHz mono int16 PCM/WAV audio")
		mode         = flag.String("mode", "s2t", "translation mode: s2t or s2s")
		sourceLang   = flag.String("source", "zh", "source language")
		targetLang   = flag.String("target", "en", "target language")
		speakerID    = flag.String("speaker", "", "speaker ID for s2s public voice; empty enables clone path when supported")
		targetFormat = flag.String("target-format", "ogg_opus", "s2s target audio format: ogg_opus or pcm")
		chunkMS      = flag.Int("chunk-ms", 80, "audio chunk duration in milliseconds")
		timeout      = flag.Duration("timeout", 90*time.Second, "overall example timeout")
	)
	flag.Parse()

	audio, err := os.ReadFile(*audioPath)
	if err != nil {
		fatalf("read audio: %v", err)
	}
	if len(audio) == 0 {
		fatalf("audio file is empty")
	}

	appID := firstEnv("DOUBAO_APP_ID", "DOUBAO_REALTIME_APP_ID")
	apiKey := firstEnv("DOUBAO_API_KEY")
	if appID == "" || apiKey == "" {
		fatalf("set DOUBAO_APP_ID and DOUBAO_API_KEY")
	}

	opts := []doubaospeech.Option{
		doubaospeech.WithResourceID(doubaospeech.ResourceASTTranslate),
		doubaospeech.WithUserID("example-ast-translate-user"),
		doubaospeech.WithAPIKey(apiKey),
	}

	client := doubaospeech.NewClient(appID, opts...)
	cfg := doubaospeech.DefaultASTTranslateConfig()
	cfg.Mode = doubaospeech.ASTTranslateMode(*mode)
	cfg.SourceLanguage = *sourceLang
	cfg.TargetLanguage = *targetLang
	cfg.SpeakerID = *speakerID
	if cfg.Mode == doubaospeech.ASTTranslateModeS2S {
		cfg.TargetAudio.Format = doubaospeech.AudioFormat(*targetFormat)
		cfg.TargetAudio.Rate = doubaospeech.SampleRate48000
		cfg.TargetAudio.Channel = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	session, err := client.ASTTranslate.OpenSession(ctx, &cfg)
	if err != nil {
		fatalf("open ast translate session: %v", err)
	}
	defer session.Close()

	done := make(chan error, 1)
	go func() {
		done <- receive(ctx, session)
	}()

	chunkSize := 16000 * 2 * *chunkMS / 1000
	if chunkSize <= 0 {
		fatalf("invalid chunk size from chunk-ms=%d", *chunkMS)
	}
	for offset := 0; offset < len(audio); offset += chunkSize {
		end := offset + chunkSize
		if end > len(audio) {
			end = len(audio)
		}
		if err := session.SendAudio(ctx, audio[offset:end]); err != nil {
			fatalf("send audio: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				fatalf("receive: %v", err)
			}
			return
		case <-ctx.Done():
			fatalf("timeout while sending audio: %v", ctx.Err())
		case <-time.After(time.Duration(*chunkMS) * time.Millisecond):
		}
	}

	if err := session.Finish(ctx); err != nil {
		fatalf("finish ast translate session: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			fatalf("receive: %v", err)
		}
	case <-ctx.Done():
		fatalf("timeout waiting for terminal event: %v", ctx.Err())
	}
}

func receive(ctx context.Context, session *doubaospeech.ASTTranslateSession) error {
	seenTranslation := false
	seenAudio := false

	for {
		evt, err := session.RecvEvent(ctx)
		if err != nil {
			return err
		}
		if evt == nil {
			return nil
		}

		switch evt.Type {
		case doubaospeech.ASTEventSourceSubtitleResponse, doubaospeech.ASTEventSourceSubtitleEnd:
			if evt.Text != "" {
				fmt.Printf("source[%d-%d]: %s\n", evt.StartTimeMS, evt.EndTimeMS, evt.Text)
			}
		case doubaospeech.ASTEventTranslationSubtitleResponse, doubaospeech.ASTEventTranslationSubtitleEnd:
			seenTranslation = true
			if evt.Text != "" {
				fmt.Printf("translation[%d-%d]: %s\n", evt.StartTimeMS, evt.EndTimeMS, evt.Text)
			}
		case doubaospeech.ASTEventTTSResponse, doubaospeech.ASTEventTTSSentenceEnd:
			if len(evt.Audio) > 0 {
				seenAudio = true
				fmt.Printf("audio: event=%d bytes=%d\n", evt.Type, len(evt.Audio))
			}
		case doubaospeech.ASTEventUsageResponse:
			if evt.Usage != nil {
				fmt.Printf("usage: duration_ms=%d items=%d\n", evt.Usage.DurationMS, len(evt.Usage.Items))
			}
		case doubaospeech.ASTEventAudioMuted:
			fmt.Printf("muted: duration_ms=%d\n", evt.MutedDurationMS)
		case doubaospeech.ASTEventSessionFinished:
			fmt.Printf("session finished: translation=%v audio=%v\n", seenTranslation, seenAudio)
			return nil
		case doubaospeech.ASTEventSessionFailed, doubaospeech.ASTEventSessionCanceled:
			if evt.Error != nil {
				return evt.Error
			}
			return fmt.Errorf("terminal event %d", evt.Type)
		}
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
