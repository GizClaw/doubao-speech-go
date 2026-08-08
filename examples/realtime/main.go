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
		speaker        string
		mode           string
		model          string
		instructions   string
		expectResponse string
		round1         string
		round2         string
		pcmPath        string
		ttsText        string
		interrupt      bool
	)

	flag.StringVar(&speaker, "speaker", strings.TrimSpace(os.Getenv("DOUBAO_REALTIME_SPEAKER")), "required TTS speaker/voice ID compatible with the selected model")
	flag.StringVar(&mode, "mode", firstNonEmpty(os.Getenv("DOUBAO_REALTIME_INPUT_MODE"), string(doubaospeech.RealtimeInputModeText)), "input mode: realtime, keep_alive, push_to_talk, text, audio_file")
	flag.StringVar(&model, "model", strings.TrimSpace(os.Getenv("DOUBAO_REALTIME_MODEL")), "required realtime model version: 1.2.1.1 or 2.2.0.0; aliases like O20 and SC20 are normalized")
	flag.StringVar(&instructions, "instructions", "You are a concise, accurate, and actionable voice assistant.", "initial system/persona instruction mapped to the selected model family")
	flag.StringVar(&expectResponse, "expect-response", "", "optional substring that must occur in the model's text response")
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
	if strings.TrimSpace(model) == "" || strings.TrimSpace(speaker) == "" {
		fmt.Fprintln(os.Stderr, "-model and -speaker are required (or set DOUBAO_REALTIME_MODEL and DOUBAO_REALTIME_SPEAKER)")
		os.Exit(2)
	}

	appID := firstNonEmpty(os.Getenv("DOUBAO_APP_ID"), os.Getenv("DOUBAO_REALTIME_APP_ID"))
	apiKey := firstNonEmpty(os.Getenv("DOUBAO_API_KEY"), os.Getenv("DOUBAO_REALTIME_API_KEY"))
	resourceID := firstNonEmpty(os.Getenv("DOUBAO_REALTIME_RESOURCE_ID"), doubaospeech.ResourceRealtime)
	searchAPIKey := strings.TrimSpace(os.Getenv("DOUBAO_VOLC_WEBSEARCH_API_KEY"))
	if appID == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "missing DOUBAO_APP_ID/DOUBAO_REALTIME_APP_ID or DOUBAO_API_KEY/DOUBAO_REALTIME_API_KEY")
		os.Exit(2)
	}

	opts := []doubaospeech.Option{
		doubaospeech.WithResourceID(resourceID),
		doubaospeech.WithUserID("example-realtime-user"),
		doubaospeech.WithAPIKey(apiKey),
	}

	client := doubaospeech.NewClient(appID, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := doubaospeech.DefaultRealtimeConfig()
	cfg.TTS.Speaker = strings.TrimSpace(speaker)
	cfg.InputMode = inputMode
	cfg.Model = modelVersion
	cfg.Instructions = instructions
	enableCustomVAD := true
	enableASRTwopass := false
	cfg.ASR.AudioInfo = &doubaospeech.RealtimeASRAudioInfo{
		Format:     doubaospeech.FormatPCM,
		SampleRate: doubaospeech.SampleRate16000,
		Channel:    1,
	}
	cfg.ASR.Extra = &doubaospeech.RealtimeASRExtra{
		EndSmoothWindowMS: 1500,
		EnableCustomVAD:   &enableCustomVAD,
		EnableASRTwopass:  &enableASRTwopass,
		Context: &doubaospeech.RealtimeASRContext{
			Hotwords: []doubaospeech.RealtimeHotword{{Word: "豆包"}, {Word: "火山引擎"}},
		},
	}
	cfg.Dialog.DialogID = strings.TrimSpace(os.Getenv("DOUBAO_REALTIME_DIALOG_ID"))
	cfg.Dialog.Location = &doubaospeech.RealtimeLocation{City: "深圳", Country: "中国", CountryCode: "CN"}
	cfg.Dialog.Extra = realtimeDialogExtra(searchAPIKey)
	cfg.TTS.AudioConfig.SpeechRate = 0
	cfg.TTS.AudioConfig.LoudnessRate = 0
	cfg.EventBuffer = 1024
	cfg.BackpressureTimeout = 30 * time.Second
	session, err := client.Realtime.OpenSession(ctx, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open realtime session: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	fmt.Printf("opened session: session_id=%s dialog_id=%s\n", session.SessionID(), session.DialogID())

	if inputMode != doubaospeech.RealtimeInputModeText {
		if err := runAudioScenario(ctx, session, inputMode, pcmPath, ttsText, interrupt, expectResponse); err != nil {
			fmt.Fprintf(os.Stderr, "audio scenario failed: %v\n", err)
			os.Exit(1)
		}
		closeSession(session)
		return
	}

	if err := runTextScenario(ctx, session, round1, round2, expectResponse); err != nil {
		fmt.Fprintf(os.Stderr, "text scenario failed: %v\n", err)
		os.Exit(1)
	}

	closeSession(session)
}

func realtimeDialogExtra(searchAPIKey string) *doubaospeech.RealtimeDialogExtra {
	enableLoudnessNorm := true
	enableUserQueryExit := true
	extra := &doubaospeech.RealtimeDialogExtra{
		AuditResponse:       "抱歉，这个问题我无法回答，你可以换个其他话题。",
		EnableLoudnessNorm:  &enableLoudnessNorm,
		EnableUserQueryExit: &enableUserQueryExit,
	}
	if searchAPIKey != "" {
		enableWebsearch := true
		extra.EnableVolcWebsearch = &enableWebsearch
		extra.VolcWebsearchType = "web"
		extra.VolcWebsearchAPIKey = searchAPIKey
		extra.VolcWebsearchResultCount = 3
		extra.VolcWebsearchNoResultMessage = "没有找到相关搜索结果。"
	}
	return extra
}

func runTextScenario(ctx context.Context, session *doubaospeech.RealtimeSession, round1, round2, expected string) error {
	if err := session.SendUserMessage(ctx, round1); err != nil {
		return fmt.Errorf("round1 send failed: %w", err)
	}

	round1Reply, err := recvUntilFinal(ctx, session, "round1")
	if err != nil {
		return fmt.Errorf("round1 receive failed: %w", err)
	}

	if err := assertExpectedResponse(round1Reply, expected); err != nil {
		return fmt.Errorf("round1 response assertion failed: %w", err)
	}

	if err := session.SendText(ctx, round2); err != nil {
		return fmt.Errorf("round2 send failed: %w", err)
	}

	if _, err := recvUntilFinalWithIterator(ctx, session, "round2"); err != nil {
		return fmt.Errorf("round2 receive failed: %w", err)
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

func runAudioScenario(ctx context.Context, session *doubaospeech.RealtimeSession, mode doubaospeech.RealtimeInputMode, pcmPath, ttsText string, interrupt bool, expected string) error {
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
		if mode != doubaospeech.RealtimeInputModePushToTalk {
			return fmt.Errorf("-interrupt requires -mode push_to_talk")
		}
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
	responseText := strings.Builder{}
	err := recvUntilEvent(ctx, session, "audio", func(evt *doubaospeech.RealtimeEvent) bool {
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
		if evt.Type == doubaospeech.EventChatResponse && evt.Text != "" {
			responseText.WriteString(evt.Text)
		}
		if expected != "" {
			return sawASR && sawASREnded && evt.Type == doubaospeech.EventChatEnded
		}
		return sawASR && sawASREnded && sawResponse
	})
	if err != nil {
		return err
	}
	return assertExpectedResponse(responseText.String(), expected)
}

func assertExpectedResponse(response, expected string) error {
	if expected == "" {
		return nil
	}
	if !strings.Contains(response, expected) {
		return fmt.Errorf("response did not contain expected substring %q", expected)
	}
	return nil
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
		end := min(offset+chunkSize, len(audio))
		if err := session.SendAudio(ctx, audio[offset:end]); err != nil {
			return fmt.Errorf("send audio chunk at offset %d failed: %w", offset, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func sendSilence(ctx context.Context, session *doubaospeech.RealtimeSession, chunks int) error {
	silence := make([]byte, 640)
	for i := range chunks {
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
