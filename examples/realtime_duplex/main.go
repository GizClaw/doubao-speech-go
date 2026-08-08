package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

const (
	pcm16KChunkBytes = 640 // 20 ms of 16 kHz mono s16le PCM.
	pcm24KChunkBytes = 960 // 20 ms of 24 kHz mono s16le PCM.
	defaultTurnWait  = 45 * time.Second
)

type exampleConfig struct {
	Rounds      int
	OutDir      string
	OldSpeaker  string
	OldModel    doubaospeech.RealtimeModelVersion
	DuplexModel string
	DuplexVoice string
	TurnTimeout time.Duration
	AppID       string
	RealtimeKey string
	DuplexKey   string
	ASRKey      string
}

type clients struct {
	realtime *doubaospeech.Client
	duplex   *doubaospeech.Client
	asr      *doubaospeech.Client
}

type oldTurnResult struct {
	Text  string
	Audio []byte
}

type duplexTurnResult struct {
	Text          string
	Audio         []byte
	Transcript    string
	FunctionCalls []doubaospeech.RealtimeDuplexFunctionCall
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Rounds)*cfg.TurnTimeout+30*time.Second)
	defer cancel()

	clients, err := newClients(cfg)
	if err != nil {
		return err
	}

	duplexCfg := doubaospeech.DefaultRealtimeDuplexConfig()
	duplexCfg.Session.Model = cfg.DuplexModel
	duplexCfg.Session.Instructions = duplexInstructions()
	duplexCfg.Session.Audio.Output.Voice = cfg.DuplexVoice
	duplexCfg.Session.Tools = demoTools()
	duplexCfg.Extension = demoDuplexExtension(cfg.DuplexVoice, strings.TrimSpace(os.Getenv("DOUBAO_VOLC_WEBSEARCH_API_KEY")))

	duplexSession, err := clients.duplex.RealtimeDuplex.OpenSession(ctx, &duplexCfg)
	if err != nil {
		return fmt.Errorf("open realtime duplex session: %w", err)
	}
	defer duplexSession.Close()
	fmt.Printf("[setup] duplex session=%s log_id=%s\n", duplexSession.SessionID(), duplexSession.LogID())

	prompt := `Please say exactly this Chinese sentence, without answering it or adding anything else: 请调用 lookup_weather 工具查询深圳今天的天气，然后用一句话回答。`
	for round := 1; round <= cfg.Rounds; round++ {
		turnCtx, cancel := context.WithTimeout(ctx, cfg.TurnTimeout)
		fmt.Printf("\n[round %d][old.prompt] %s\n", round, prompt)
		oldResult, err := runOldRealtimeTurn(turnCtx, clients.realtime, cfg, prompt)
		cancel()
		if err != nil {
			return fmt.Errorf("round %d old realtime turn: %w", round, err)
		}
		fmt.Printf("[round %d][old.text] %s\n", round, oldResult.Text)
		fmt.Printf("[round %d][old.audio] %d bytes pcm16k\n", round, len(oldResult.Audio))

		turnCtx, cancel = context.WithTimeout(ctx, cfg.TurnTimeout)
		duplexResult, err := runDuplexTurn(turnCtx, duplexSession, oldResult.Audio)
		cancel()
		if err != nil {
			return fmt.Errorf("round %d duplex turn: %w", round, err)
		}
		fmt.Printf("[round %d][duplex.text] %s\n", round, duplexResult.Text)
		fmt.Printf("[round %d][duplex.audio] %d bytes pcm24k\n", round, len(duplexResult.Audio))
		for _, call := range duplexResult.FunctionCalls {
			fmt.Printf("[round %d][duplex.function_call] name=%s call_id=%s arguments=%s\n", round, call.Name, call.CallID, call.Arguments)
		}

		turnCtx, cancel = context.WithTimeout(ctx, cfg.TurnTimeout)
		transcript, err := transcribePCM(turnCtx, clients.asr, duplexResult.Audio, doubaospeech.SampleRate24000)
		cancel()
		if err != nil {
			fmt.Printf("[round %d][asr.warning] transcript skipped: %v\n", round, err)
			transcript = duplexResult.Text
		} else {
			fmt.Printf("[round %d][asr.transcript] %s\n", round, transcript)
		}
		duplexResult.Transcript = transcript

		if cfg.OutDir != "" {
			if err := writeRoundArtifacts(cfg.OutDir, round, oldResult.Audio, duplexResult.Audio, transcript); err != nil {
				return err
			}
		}

		prompt = nextOldPrompt(round, oldResult.Text, transcript)
	}

	return nil
}

func parseConfig(args []string) (exampleConfig, error) {
	cfg := exampleConfig{}
	var turnTimeout time.Duration
	fs := flag.NewFlagSet("realtime_duplex", flag.ContinueOnError)
	fs.IntVar(&cfg.Rounds, "rounds", 2, "number of old-realtime to duplex dialogue rounds")
	fs.StringVar(&cfg.OutDir, "out-dir", "", "optional directory for old/duplex PCM artifacts and transcripts")
	fs.StringVar(&cfg.OldSpeaker, "old-speaker", firstNonEmpty(os.Getenv("DOUBAO_REALTIME_SPEAKER"), "zh_female_cancan"), "old realtime TTS speaker")
	fs.StringVar((*string)(&cfg.OldModel), "old-model", firstNonEmpty(os.Getenv("DOUBAO_REALTIME_MODEL"), string(doubaospeech.RealtimeModelO20)), "old realtime model")
	fs.StringVar(&cfg.DuplexModel, "duplex-model", firstNonEmpty(os.Getenv("DOUBAO_DUPLEX_MODEL"), doubaospeech.RealtimeDuplexModelDefault), "duplex model")
	fs.StringVar(&cfg.DuplexVoice, "duplex-voice", firstNonEmpty(os.Getenv("DOUBAO_DUPLEX_VOICE"), "zh_male_xiaotian_jupiter_bigtts"), "duplex output voice")
	fs.DurationVar(&turnTimeout, "turn-timeout", defaultTurnWait, "per-turn timeout")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.Rounds <= 0 {
		return cfg, errors.New("-rounds must be positive")
	}
	cfg.TurnTimeout = turnTimeout
	cfg.AppID = firstNonEmpty(os.Getenv("DOUBAO_APP_ID"), os.Getenv("DOUBAO_REALTIME_APP_ID"))
	cfg.RealtimeKey = firstNonEmpty(os.Getenv("DOUBAO_REALTIME_API_KEY"), os.Getenv("DOUBAO_API_KEY"))
	cfg.DuplexKey = firstNonEmpty(
		os.Getenv("DOUBAO_DUPLEX_API_KEY"),
		os.Getenv("DOUBAO_API_KEY"),
		os.Getenv("DOUBAO_REALTIME_API_KEY"),
	)
	cfg.ASRKey = firstNonEmpty(
		os.Getenv("DOUBAO_ASR_API_KEY"),
		os.Getenv("DOUBAO_API_KEY"),
		os.Getenv("DOUBAO_REALTIME_API_KEY"),
	)
	return cfg, validateConfig(cfg)
}

func validateConfig(cfg exampleConfig) error {
	if cfg.AppID == "" {
		return errors.New("DOUBAO_APP_ID or DOUBAO_REALTIME_APP_ID is required")
	}
	if cfg.RealtimeKey == "" {
		return errors.New("DOUBAO_REALTIME_API_KEY or DOUBAO_API_KEY is required")
	}
	if cfg.DuplexKey == "" {
		return errors.New("DOUBAO_DUPLEX_API_KEY or DOUBAO_API_KEY is required")
	}
	if cfg.ASRKey == "" {
		return errors.New("DOUBAO_ASR_API_KEY or DOUBAO_API_KEY is required")
	}
	return nil
}

func newClients(cfg exampleConfig) (*clients, error) {
	realtimeOpts := []doubaospeech.Option{
		doubaospeech.WithResourceID(doubaospeech.ResourceRealtime),
		doubaospeech.WithUserID("example-realtime-duplex-old"),
		doubaospeech.WithAPIKey(cfg.RealtimeKey),
	}

	asrOpts := []doubaospeech.Option{
		doubaospeech.WithResourceID(doubaospeech.ResourceASRStreamV2),
		doubaospeech.WithUserID("example-realtime-duplex-asr"),
		doubaospeech.WithAPIKey(cfg.ASRKey),
	}

	duplexOpts := []doubaospeech.Option{
		doubaospeech.WithUserID("example-realtime-duplex"),
		doubaospeech.WithAPIKey(cfg.DuplexKey),
	}

	return &clients{
		realtime: doubaospeech.NewClient(cfg.AppID, realtimeOpts...),
		duplex:   doubaospeech.NewClient(cfg.AppID, duplexOpts...),
		asr:      doubaospeech.NewClient(cfg.AppID, asrOpts...),
	}, nil
}

func runOldRealtimeTurn(ctx context.Context, client *doubaospeech.Client, cfg exampleConfig, prompt string) (oldTurnResult, error) {
	rtCfg := doubaospeech.DefaultRealtimeConfig()
	rtCfg.TTS.Speaker = cfg.OldSpeaker
	rtCfg.TTS.AudioConfig.Format = doubaospeech.FormatPCMS16LE
	rtCfg.TTS.AudioConfig.SampleRate = doubaospeech.SampleRate24000
	rtCfg.TTS.AudioConfig.Bits = 16
	rtCfg.InputMode = doubaospeech.RealtimeInputModeText
	rtCfg.Model = cfg.OldModel
	rtCfg.Instructions = "You are the first assistant in an integration smoke test. Speak exactly the requested sentence."
	rtCfg.EventBuffer = 1024
	rtCfg.BackpressureTimeout = 30 * time.Second
	rtCfg.Props = doubaospeech.RealtimeGenerationProps{Temperature: 0.1, TopP: 0.8, MaxTokens: 128}

	session, err := client.Realtime.OpenSession(ctx, &rtCfg)
	if err != nil {
		return oldTurnResult{}, err
	}
	defer session.Close()

	if err := session.SendUserMessage(ctx, prompt); err != nil {
		return oldTurnResult{}, err
	}

	var result oldTurnResult
	for {
		evt, err := session.RecvEvent(ctx)
		if err != nil {
			return oldTurnResult{}, err
		}
		if evt.Text != "" {
			fmt.Printf("[old.event=%d][text] %s\n", evt.Type, evt.Text)
			result.Text += evt.Text
		}
		if len(evt.Audio) > 0 {
			fmt.Printf("[old.event=%d][audio] %d bytes\n", evt.Type, len(evt.Audio))
			result.Audio = append(result.Audio, evt.Audio...)
		}
		if evt.Type == doubaospeech.EventTTSFinished || evt.Type == doubaospeech.EventSessionFinished {
			break
		}
	}
	if len(result.Audio) == 0 {
		return result, errors.New("old realtime returned no audio")
	}
	return result, nil
}

func runDuplexTurn(ctx context.Context, session *doubaospeech.RealtimeDuplexSession, inputPCM16K []byte) (duplexTurnResult, error) {
	for _, chunk := range chunkAudio(inputPCM16K, pcm16KChunkBytes) {
		if err := session.SendAudio(ctx, chunk); err != nil {
			return duplexTurnResult{}, err
		}
		if err := sleepContext(ctx, 20*time.Millisecond); err != nil {
			return duplexTurnResult{}, err
		}
	}
	if err := session.CommitAudio(ctx); err != nil {
		return duplexTurnResult{}, err
	}

	silenceCtx, stopSilence := context.WithCancel(ctx)
	defer stopSilence()
	go sendDuplexSilence(silenceCtx, session)

	var result duplexTurnResult
	for {
		evt, err := session.RecvEvent(ctx)
		if err != nil {
			return result, err
		}
		printDuplexEvent(evt)
		switch evt.Type {
		case doubaospeech.RealtimeDuplexEventResponseFunctionCallArgumentsDone:
			result.FunctionCalls = append(result.FunctionCalls, evt.FunctionCalls...)
			outputs := make([]doubaospeech.RealtimeDuplexFunctionCallOutput, 0, len(evt.FunctionCalls))
			for _, call := range evt.FunctionCalls {
				outputs = append(outputs, doubaospeech.RealtimeDuplexFunctionCallOutput{
					CallID: call.CallID,
					Output: demoToolOutput(call),
				})
			}
			if len(outputs) > 0 {
				if err := session.SendFunctionCallOutputs(ctx, outputs...); err != nil {
					return result, err
				}
			}
		case doubaospeech.RealtimeDuplexEventResponseOutputTextDelta:
			result.Text += evt.Delta
		case doubaospeech.RealtimeDuplexEventResponseOutputTextDone:
			if evt.Text != "" {
				result.Text = evt.Text
			}
		case doubaospeech.RealtimeDuplexEventResponseOutputAudioDelta:
			result.Audio = append(result.Audio, evt.Audio...)
		case doubaospeech.RealtimeDuplexEventResponseOutputAudioDone, doubaospeech.RealtimeDuplexEventSessionClosed:
			if len(result.Audio) == 0 {
				return result, errors.New("duplex returned no audio")
			}
			return result, nil
		}
	}
}

func transcribePCM(ctx context.Context, client *doubaospeech.Client, audio []byte, sampleRate doubaospeech.SampleRate) (string, error) {
	if len(audio) == 0 {
		return "", errors.New("audio is empty")
	}
	session, err := client.ASRV2.OpenStreamSession(ctx, &doubaospeech.ASRV2Config{
		Format:     doubaospeech.FormatPCM,
		SampleRate: sampleRate,
		ResultType: "full",
	})
	if err != nil {
		return "", err
	}
	defer session.Close()

	chunkSize := pcm24KChunkBytes * 5
	if sampleRate == doubaospeech.SampleRate16000 {
		chunkSize = pcm16KChunkBytes * 5
	}
	chunks := chunkAudio(audio, chunkSize)
	for i, chunk := range chunks {
		if err := session.SendAudio(ctx, chunk, i+1 == len(chunks)); err != nil {
			return "", err
		}
	}

	var lastText string
	for result, err := range session.Recv() {
		if err != nil {
			return "", err
		}
		if result.Text != "" {
			fmt.Printf("[asr.event][final=%v] %s\n", result.IsFinal, result.Text)
			lastText = result.Text
		}
		if result.IsFinal {
			return strings.TrimSpace(lastText), nil
		}
	}
	if strings.TrimSpace(lastText) == "" {
		return "", errors.New("asr returned empty transcript")
	}
	return strings.TrimSpace(lastText), nil
}

func sendDuplexSilence(ctx context.Context, session *doubaospeech.RealtimeDuplexSession) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	silence := make([]byte, pcm16KChunkBytes)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = session.SendAudio(ctx, silence)
		}
	}
}

func demoTools() []doubaospeech.RealtimeDuplexFunctionTool {
	additionalProperties := false
	return []doubaospeech.RealtimeDuplexFunctionTool{
		{
			Type:        "function",
			Name:        "lookup_weather",
			Description: "Look up a deterministic weather summary for a city. Use this tool when the user asks to query weather.",
			Parameters: &doubaospeech.RealtimeDuplexJSONSchema{
				Type: "object",
				Properties: map[string]*doubaospeech.RealtimeDuplexJSONSchema{
					"city": {Type: "string", Description: "City name, for example 深圳"},
				},
				Required:             []string{"city"},
				AdditionalProperties: &additionalProperties,
			},
		},
	}
}

func demoDuplexExtension(voice string, searchAPIKey string) *doubaospeech.RealtimeDuplexExtension {
	enableLoudnessNorm := true
	enableMusic := false
	enableUserQueryExit := true
	enableASRTwopass := false
	extra := &doubaospeech.RealtimeDuplexDialogExtra{
		AuditResponse:       "抱歉，这个问题我无法回答，你可以换个其他话题。",
		EnableLoudnessNorm:  &enableLoudnessNorm,
		EnableMusic:         &enableMusic,
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
	return &doubaospeech.RealtimeDuplexExtension{
		ASR: &doubaospeech.RealtimeASRConfig{
			AudioInfo: &doubaospeech.RealtimeASRAudioInfo{
				Format:     doubaospeech.FormatPCM,
				SampleRate: doubaospeech.SampleRate16000,
				Channel:    1,
			},
			Extra: &doubaospeech.RealtimeASRExtra{
				EnableASRTwopass: &enableASRTwopass,
				Context: &doubaospeech.RealtimeASRContext{
					Hotwords: []doubaospeech.RealtimeHotword{{Word: "lookup_weather"}, {Word: "深圳"}},
				},
			},
		},
		TTS: &doubaospeech.RealtimeTTSConfig{
			Speaker: strings.TrimSpace(voice),
			AudioConfig: doubaospeech.RealtimeAudioConfig{
				SpeechRate:   0,
				LoudnessRate: 0,
			},
		},
		Dialog: &doubaospeech.RealtimeDuplexDialogExtension{
			Location: &doubaospeech.RealtimeLocation{City: "深圳", Country: "中国", CountryCode: "CN"},
			Extra:    extra,
		},
	}
}

func demoToolOutput(call doubaospeech.RealtimeDuplexFunctionCall) string {
	var args map[string]any
	_ = json.Unmarshal([]byte(call.Arguments), &args)
	city, _ := args["city"].(string)
	if strings.TrimSpace(city) == "" {
		city = "深圳"
	}
	switch call.Name {
	case "lookup_weather":
		return fmt.Sprintf(`{"city":%q,"summary":"晴到多云，适合进行语音对话测试。","temperature_c":26}`, city)
	default:
		return fmt.Sprintf(`{"error":"unsupported tool %s"}`, call.Name)
	}
}

func duplexInstructions() string {
	return strings.Join([]string{
		"You are the second assistant in a live SDK integration test.",
		"When the user asks to call lookup_weather, you must call the lookup_weather tool before answering.",
		"After receiving tool output, answer in one concise Chinese sentence.",
	}, " ")
}

func nextOldPrompt(round int, oldText string, duplexTranscript string) string {
	return fmt.Sprintf(
		"Please say exactly this Chinese sentence, without answering it or adding anything else: 上一轮我说的是“%s”，对方回答的是“%s”。请继续要求对方调用 lookup_weather 工具，并让对方用一句话回答第 %d 轮结果。",
		compactForPrompt(oldText),
		compactForPrompt(duplexTranscript),
		round+1,
	)
}

func compactForPrompt(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len([]rune(text)) <= 80 {
		return text
	}
	runes := []rune(text)
	return string(runes[:80])
}

func writeRoundArtifacts(outDir string, round int, oldAudio, duplexAudio []byte, transcript string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("round%d-old-realtime.pcm", round)), oldAudio, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("round%d-duplex.pcm", round)), duplexAudio, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, fmt.Sprintf("round%d-duplex-asr.txt", round)), []byte(transcript+"\n"), 0644)
}

func chunkAudio(data []byte, size int) [][]byte {
	if size <= 0 {
		return nil
	}
	chunks := make([][]byte, 0, (len(data)+size-1)/size)
	for len(data) > 0 {
		n := min(len(data), size)
		chunks = append(chunks, data[:n])
		data = data[n:]
	}
	return chunks
}

func printDuplexEvent(evt *doubaospeech.RealtimeDuplexEvent) {
	switch evt.Type {
	case doubaospeech.RealtimeDuplexEventTranscriptionDelta:
		fmt.Printf("[duplex.asr.delta] %s\n", evt.Delta)
	case doubaospeech.RealtimeDuplexEventTranscriptionCompleted:
		fmt.Printf("[duplex.asr.done] %s\n", evt.Transcript)
	case doubaospeech.RealtimeDuplexEventResponseFunctionCallArgumentsDone:
		for _, call := range evt.FunctionCalls {
			fmt.Printf("[duplex.function_call.raw] name=%s call_id=%s arguments=%s\n", call.Name, call.CallID, call.Arguments)
		}
	case doubaospeech.RealtimeDuplexEventResponseOutputTextDelta:
		fmt.Printf("[duplex.text.delta] %s\n", evt.Delta)
	case doubaospeech.RealtimeDuplexEventResponseOutputTextDone:
		fmt.Printf("[duplex.text.done] %s\n", evt.Text)
	case doubaospeech.RealtimeDuplexEventResponseOutputAudioDelta:
		fmt.Printf("[duplex.audio.delta] %d bytes\n", len(evt.Audio))
	case doubaospeech.RealtimeDuplexEventResponseOutputAudioDone:
		fmt.Printf("[duplex.audio.done] response=%s status=%s\n", evt.ResponseID, evt.StatusCode)
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
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
