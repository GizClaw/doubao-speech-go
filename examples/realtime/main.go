package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

func main() {
	var (
		speaker   string
		mode      string
		model     string
		round1    string
		round2    string
		pcmPath   string
		ttsText   string
		interrupt bool
	)

	flag.StringVar(&speaker, "speaker", firstNonEmpty(os.Getenv("DOUBAO_REALTIME_SPEAKER"), "zh_female_cancan"), "TTS speaker/voice ID")
	flag.StringVar(&mode, "mode", firstNonEmpty(os.Getenv("DOUBAO_REALTIME_INPUT_MODE"), string(doubaospeech.RealtimeInputModeText)), "input mode: realtime, keep_alive, push_to_talk, text, audio_file")
	flag.StringVar(&model, "model", firstNonEmpty(os.Getenv("DOUBAO_REALTIME_MODEL"), string(doubaospeech.RealtimeModelO20)), "realtime model version: 1.2.1.1 or 2.2.0.0; legacy aliases like O and SC are normalized")
	flag.StringVar(&round1, "round1", "Please give a brief self-introduction.", "First-round user message")
	flag.StringVar(&round2, "round2", "Based on the updated settings, summarize your capability boundaries in two sentences.", "Second-round user message")
	flag.StringVar(&pcmPath, "pcm", "examples/asr_v2_sauc_ws/sample_zh_16k.pcm", "16 kHz mono int16 PCM file for audio-mode smoke")
	flag.StringVar(&ttsText, "tts-text", "", "optional ChatTTSText smoke text after audio ASR end")
	flag.BoolVar(&interrupt, "interrupt", false, "interrupt the first audio response and verify the session remains usable")
	flag.Parse()

	inputMode, err := parseRealtimeInputMode(mode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	modelVersion := doubaospeech.RealtimeModelVersion(strings.TrimSpace(model))

	appID := firstNonEmpty(os.Getenv("DOUBAO_APP_ID"), os.Getenv("DOUBAO_REALTIME_APP_ID"))
	apiKey := firstNonEmpty(os.Getenv("DOUBAO_API_KEY"), os.Getenv("DOUBAO_REALTIME_API_KEY"))
	accessKey := firstNonEmpty(os.Getenv("DOUBAO_ACCESS_KEY"), os.Getenv("DOUBAO_REALTIME_ACCESS_KEY"))
	resourceID := firstNonEmpty(os.Getenv("DOUBAO_REALTIME_RESOURCE_ID"), doubaospeech.ResourceRealtime)
	if apiKey == "" && accessKey == "" {
		fmt.Fprintln(os.Stderr, "missing DOUBAO_API_KEY/DOUBAO_REALTIME_API_KEY or DOUBAO_ACCESS_KEY/DOUBAO_REALTIME_ACCESS_KEY")
		os.Exit(2)
	}
	if apiKey == "" && appID == "" {
		fmt.Fprintln(os.Stderr, "missing environment variable DOUBAO_APP_ID or DOUBAO_REALTIME_APP_ID for app-id auth")
		os.Exit(2)
	}

	opts := []doubaospeech.Option{
		doubaospeech.WithResourceID(resourceID),
		doubaospeech.WithUserID("example-realtime-user"),
	}
	if apiKey != "" {
		opts = append(opts, doubaospeech.WithAPIKey(apiKey))
	} else {
		opts = append(opts, doubaospeech.WithAppID(appID, accessKey, doubaospeech.AppKeyRealtime))
	}

	client := doubaospeech.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := doubaospeech.DefaultRealtimeConfig()
	cfg.TTS.Speaker = strings.TrimSpace(speaker)
	cfg.InputMode = inputMode
	cfg.Model = modelVersion
	cfg.EventBuffer = 1024
	cfg.BackpressureTimeout = 30 * time.Second
	cfg.Prompt = doubaospeech.RealtimePromptConfig{
		System: "You are a concise, accurate, and actionable voice assistant.",
		Variables: map[string]string{
			"tone": "professional",
		},
	}
	cfg.Props = doubaospeech.RealtimeGenerationProps{
		Temperature: 0.3,
		TopP:        0.9,
		MaxTokens:   256,
	}

	session, err := client.Realtime.OpenSession(ctx, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open realtime session: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	fmt.Printf("opened session: %s\n", session.SessionID())

	if inputMode != doubaospeech.RealtimeInputModeText {
		if err := runAudioScenario(ctx, session, inputMode, pcmPath, ttsText, interrupt); err != nil {
			fmt.Fprintf(os.Stderr, "audio scenario failed: %v\n", err)
			os.Exit(1)
		}
		closeSession(session)
		return
	}

	if err := runTextScenario(ctx, session, round1, round2); err != nil {
		fmt.Fprintf(os.Stderr, "text scenario failed: %v\n", err)
		os.Exit(1)
	}

	closeSession(session)
}

func runTextScenario(ctx context.Context, session *doubaospeech.RealtimeSession, round1, round2 string) error {
	if err := session.SendUserMessage(ctx, round1); err != nil {
		return fmt.Errorf("round1 send failed: %w", err)
	}

	round1Reply, err := recvUntilFinal(ctx, session, "round1")
	if err != nil {
		return fmt.Errorf("round1 receive failed: %w", err)
	}

	// Multi-turn update 1: rewrite history before round 2.
	session.UpdateHistory([]doubaospeech.RealtimeConversationMessage{
		{Role: "user", Content: round1},
		{Role: "assistant", Content: round1Reply + " (history revised in example before round 2)"},
	})
	if err := session.ReplaceHistory(1, doubaospeech.RealtimeConversationMessage{
		Role:    "assistant",
		Content: round1Reply + " (ReplaceHistory: second revision before round 2)",
	}); err != nil {
		return fmt.Errorf("replace history failed: %w", err)
	}

	// Multi-turn update 2: update prompt.
	session.UpdatePrompt(doubaospeech.RealtimePromptConfig{
		System: "Now state limitations more explicitly and keep the answer within two sentences.",
		Variables: map[string]string{
			"tone": "concise",
		},
	})

	// Multi-turn update 3: update generation props.
	session.UpdateProps(doubaospeech.RealtimeGenerationProps{
		Temperature: 0.1,
		TopP:        0.8,
		MaxTokens:   128,
	})

	if err := session.SendText(ctx, round2); err != nil {
		return fmt.Errorf("round2 send failed: %w", err)
	}

	if _, err := recvUntilFinalWithIterator(ctx, session, "round2"); err != nil {
		return fmt.Errorf("round2 receive failed: %w", err)
	}

	if err := session.Interrupt(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "interrupt returned error (may be expected on some servers): %v\n", err)
	}
	return nil
}

func closeSession(session *doubaospeech.RealtimeSession) {
	if err := session.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close failed: %v\n", err)
		os.Exit(1)
	}
	if err := session.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "second close failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("session closed idempotently")
}

func runAudioScenario(ctx context.Context, session *doubaospeech.RealtimeSession, mode doubaospeech.RealtimeInputMode, pcmPath, ttsText string, interrupt bool) error {
	if ttsText != "" {
		if err := sendAudioTurn(ctx, session, mode, pcmPath); err != nil {
			return err
		}
		if err := recvUntilEvent(ctx, session, "audio-asr", func(evt *doubaospeech.RealtimeEvent) bool {
			return evt.Type == doubaospeech.EventASREnded
		}); err != nil {
			return err
		}
		if err := session.SendTTSText(ctx, ttsText); err != nil {
			return fmt.Errorf("send tts text failed: %w", err)
		}
		return recvUntilEvent(ctx, session, "tts-text", func(evt *doubaospeech.RealtimeEvent) bool {
			return evt.Type == doubaospeech.EventTTSAudioData && len(evt.Audio) > 0
		})
	}

	if err := sendAudioTurn(ctx, session, mode, pcmPath); err != nil {
		return err
	}

	if interrupt {
		if err := recvUntilEvent(ctx, session, "audio-interrupt", func(evt *doubaospeech.RealtimeEvent) bool {
			return evt.Type == doubaospeech.EventChatResponse ||
				evt.Type == doubaospeech.EventTTSStarted ||
				evt.Type == doubaospeech.EventTTSAudioData
		}); err != nil {
			return err
		}
		if err := session.Interrupt(ctx); err != nil {
			return fmt.Errorf("interrupt failed: %w", err)
		}
		fmt.Println("sent ClientInterrupt")
		if err := sendAudioTurn(ctx, session, mode, pcmPath); err != nil {
			return err
		}
		return recvUntilEvent(ctx, session, "audio-after-interrupt", func(evt *doubaospeech.RealtimeEvent) bool {
			return evt.Type == doubaospeech.EventASRResponse && strings.TrimSpace(evt.Text) != ""
		})
	}

	var sawASR, sawASREnded, sawResponse bool
	return recvUntilEvent(ctx, session, "audio", func(evt *doubaospeech.RealtimeEvent) bool {
		if evt.Type == doubaospeech.EventASRResponse && strings.TrimSpace(evt.Text) != "" {
			sawASR = true
		}
		if evt.Type == doubaospeech.EventASREnded {
			sawASREnded = true
		}
		if evt.Type == doubaospeech.EventChatResponse ||
			evt.Type == doubaospeech.EventTTSStarted ||
			evt.Type == doubaospeech.EventTTSAudioData {
			sawResponse = true
		}
		return sawASR && sawASREnded && sawResponse
	})
}

func sendAudioTurn(ctx context.Context, session *doubaospeech.RealtimeSession, mode doubaospeech.RealtimeInputMode, pcmPath string) error {
	if err := sendPCMFile(ctx, session, pcmPath); err != nil {
		return err
	}
	switch mode {
	case doubaospeech.RealtimeInputModePushToTalk:
		if err := session.EndASR(ctx); err != nil {
			return fmt.Errorf("end asr failed: %w", err)
		}
	case doubaospeech.RealtimeInputModeDefault:
		if err := sendSilence(ctx, session, 150); err != nil {
			return err
		}
	}
	return nil
}

func sendPCMFile(ctx context.Context, session *doubaospeech.RealtimeSession, path string) error {
	audio, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read pcm file failed: %w", err)
	}
	const chunkSize = 640
	for offset := 0; offset < len(audio); offset += chunkSize {
		end := offset + chunkSize
		if end > len(audio) {
			end = len(audio)
		}
		if err := session.SendAudio(ctx, audio[offset:end]); err != nil {
			return fmt.Errorf("send audio chunk at offset %d failed: %w", offset, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func sendSilence(ctx context.Context, session *doubaospeech.RealtimeSession, chunks int) error {
	silence := make([]byte, 640)
	for i := 0; i < chunks; i++ {
		if err := session.SendAudio(ctx, silence); err != nil {
			return fmt.Errorf("send silence chunk %d failed: %w", i, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func recvUntilEvent(ctx context.Context, session *doubaospeech.RealtimeSession, label string, done func(*doubaospeech.RealtimeEvent) bool) error {
	for {
		evt, err := session.RecvEvent(ctx)
		if err != nil {
			return err
		}
		if evt == nil {
			return fmt.Errorf("stream closed before expected event in %s", label)
		}
		printRealtimeEvent(label, evt)
		if done(evt) {
			return nil
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseRealtimeInputMode(mode string) (doubaospeech.RealtimeInputMode, error) {
	switch strings.TrimSpace(mode) {
	case "", "realtime", "default":
		return doubaospeech.RealtimeInputModeDefault, nil
	case string(doubaospeech.RealtimeInputModeKeepAlive):
		return doubaospeech.RealtimeInputModeKeepAlive, nil
	case string(doubaospeech.RealtimeInputModePushToTalk):
		return doubaospeech.RealtimeInputModePushToTalk, nil
	case string(doubaospeech.RealtimeInputModeText):
		return doubaospeech.RealtimeInputModeText, nil
	case string(doubaospeech.RealtimeInputModeAudioFile):
		return doubaospeech.RealtimeInputModeAudioFile, nil
	default:
		return "", fmt.Errorf("unsupported -mode %q", mode)
	}
}

func printRealtimeEvent(label string, evt *doubaospeech.RealtimeEvent) {
	if evt == nil {
		return
	}
	if evt.Text != "" {
		fmt.Printf("[%s][event=%d][final=%v] %s\n", label, evt.Type, evt.IsFinal, evt.Text)
		return
	}
	if len(evt.Audio) > 0 {
		fmt.Printf("[%s][event=%d][final=%v] audio=%d bytes\n", label, evt.Type, evt.IsFinal, len(evt.Audio))
		return
	}
	fmt.Printf("[%s][event=%d][final=%v]\n", label, evt.Type, evt.IsFinal)
}

func recvUntilFinal(ctx context.Context, session *doubaospeech.RealtimeSession, round string) (string, error) {
	builder := strings.Builder{}

	for {
		evt, err := session.RecvEvent(ctx)
		if err != nil {
			return "", err
		}
		if evt == nil {
			return "", fmt.Errorf("stream closed before final in %s", round)
		}

		switch evt.Type {
		case doubaospeech.EventChatResponse, doubaospeech.EventChatEnded, doubaospeech.EventTTSSegmentEnd, doubaospeech.EventTTSFinished:
			if evt.Text != "" {
				builder.WriteString(evt.Text)
				fmt.Printf("[%s][event=%d][final=%v] %s\n", round, evt.Type, evt.IsFinal, evt.Text)
			}
		default:
			if evt.Text != "" {
				fmt.Printf("[%s][event=%d][final=%v] %s\n", round, evt.Type, evt.IsFinal, evt.Text)
			}
		}

		if evt.IsFinal {
			break
		}
	}

	return builder.String(), nil
}

func recvUntilFinalWithIterator(ctx context.Context, session *doubaospeech.RealtimeSession, round string) (string, error) {
	builder := strings.Builder{}
	type recvItem struct {
		evt *doubaospeech.RealtimeEvent
		err error
	}

	itemCh := make(chan recvItem, 1)
	stopCh := make(chan struct{})
	go func() {
		defer close(itemCh)
		for evt, err := range session.Recv() {
			item := recvItem{evt: evt, err: err}
			select {
			case <-stopCh:
				return
			case itemCh <- item:
			}
			if err != nil {
				return
			}
		}
	}()
	defer close(stopCh)

	for {
		select {
		case <-ctx.Done():
			_ = session.Close()
			return "", ctx.Err()
		case item, ok := <-itemCh:
			if !ok {
				return "", fmt.Errorf("stream closed before final in %s", round)
			}
			if item.err != nil {
				return "", item.err
			}
			evt := item.evt
			if evt == nil {
				continue
			}

			switch evt.Type {
			case doubaospeech.EventChatResponse, doubaospeech.EventChatEnded, doubaospeech.EventTTSSegmentEnd, doubaospeech.EventTTSFinished:
				if evt.Text != "" {
					builder.WriteString(evt.Text)
					fmt.Printf("[%s][event=%d][final=%v] %s\n", round, evt.Type, evt.IsFinal, evt.Text)
				}
			default:
				if evt.Text != "" {
					fmt.Printf("[%s][event=%d][final=%v] %s\n", round, evt.Type, evt.IsFinal, evt.Text)
				}
			}

			if evt.IsFinal {
				return builder.String(), nil
			}
		}
	}
}
