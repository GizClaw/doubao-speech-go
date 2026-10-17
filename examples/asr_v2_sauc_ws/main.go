package main

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

//go:embed sample_zh_16k.pcm
var embeddedSampleAudio []byte

type sendResult struct {
	duration time.Duration
	eosAt    time.Time
	err      error
}

type copyTranscript struct {
	startMS int64
	endMS   int64
	text    string
}

type continuousProgress struct {
	mu     sync.Mutex
	copies []copyTranscript
}

func newContinuousProgress(turns int, duration time.Duration) *continuousProgress {
	p := &continuousProgress{copies: make([]copyTranscript, turns)}
	for i := range turns {
		p.copies[i].startMS = (time.Duration(i) * duration).Milliseconds()
		p.copies[i].endMS = (time.Duration(i+1) * duration).Milliseconds()
	}
	return p
}

func (p *continuousProgress) record(utterance doubaospeech.ASRV2Utterance) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.copies {
		copy := &p.copies[i]
		if int64(utterance.StartTime) >= copy.startMS && int64(utterance.EndTime) <= copy.endMS {
			copy.text += utterance.Text
			return
		}
	}
}

func (p *continuousProgress) check(turn int, expect string) error {
	p.mu.Lock()
	text := p.copies[turn].text
	p.mu.Unlock()
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("continuous input %d did not produce a definite utterance before audio EOS", turn+1)
	}
	if expect != "" && !strings.Contains(text, expect) {
		return fmt.Errorf("continuous input %d transcript %q does not contain %q", turn+1, text, expect)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	audioPath := flag.String("audio", "", "Local 16 kHz mono signed 16-bit PCM file; defaults to the embedded sample")
	sampleRate := flag.Int("sample-rate", 16000, "PCM sample rate (16000)")
	format := flag.String("format", "pcm", "Audio format (pcm)")
	chunkSize := flag.Int("chunk-size", 3200, "Bytes per paced audio packet")
	resourceID := flag.String("resource-id", doubaospeech.ResourceASRStreamV2, "ASR resource ID")
	mode := flag.String("mode", "ptt", "ptt: finalize after audio; continuous: require utterance boundaries before audio EOS")
	turns := flag.Int("turns", 2, "Number of copies of the input in one continuous session")
	silence := flag.Duration("silence", 2*time.Second, "Silence after each continuous input copy")
	endWindowSize := flag.Int("end-window-size", 200, "VAD silence window in milliseconds")
	forceToSpeechTime := flag.Int("force-to-speech-time", 1000, "Minimum audio duration before VAD endpointing in milliseconds")
	expect := flag.String("expect", "", "Required substring for each input copy in the final transcript")
	flag.Parse()

	if strings.ToLower(strings.TrimSpace(*format)) != "pcm" || *sampleRate != 16000 {
		return fmt.Errorf("this example requires 16 kHz mono signed 16-bit PCM")
	}
	if *chunkSize <= 0 || *chunkSize%2 != 0 {
		return fmt.Errorf("chunk-size must be a positive number of complete 16-bit samples")
	}
	if *mode != "ptt" && *mode != "continuous" {
		return fmt.Errorf("mode must be ptt or continuous")
	}
	if *turns < 1 || *silence <= 0 || *silence > 10*time.Second {
		return fmt.Errorf("turns must be positive and silence must be within (0, 10s]")
	}
	appID, apiKey := os.Getenv("DOUBAO_APP_ID"), os.Getenv("DOUBAO_API_KEY")
	if appID == "" || apiKey == "" {
		return fmt.Errorf("missing DOUBAO_APP_ID or DOUBAO_API_KEY")
	}
	audio := embeddedSampleAudio
	if *audioPath != "" {
		var err error
		audio, err = os.ReadFile(*audioPath)
		if err != nil {
			return fmt.Errorf("read audio: %w", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "using embedded sample: sample_zh_16k.pcm")
	}
	if isLFSPointerContent(audio) {
		return fmt.Errorf("audio is a Git LFS pointer; run git lfs pull or pass -audio")
	}
	if len(audio) == 0 || len(audio)%2 != 0 {
		return fmt.Errorf("audio must contain complete 16-bit PCM samples")
	}

	client := doubaospeech.NewClient(appID,
		doubaospeech.WithResourceID(*resourceID),
		doubaospeech.WithUserID("asr-example"),
		doubaospeech.WithAPIKey(apiKey),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	falseValue := false
	session, err := client.ASRV2.OpenStreamSession(ctx, &doubaospeech.ASRV2Config{
		Format:     doubaospeech.FormatPCM,
		SampleRate: doubaospeech.SampleRate16000,
		Request: &doubaospeech.ASRV2RequestConfig{
			EnableNonstream:   &falseValue,
			ResultType:        "full",
			EndWindowSize:     endWindowSize,
			ForceToSpeechTime: forceToSpeechTime,
		},
	})
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer session.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stopClose()
	fmt.Printf("mode=%s resource=%s endpoint=bigmodel_async enable_nonstream=false\n", *mode, *resourceID)

	quiet := make([]byte, int(*silence*32000/time.Second)/2*2)
	copyDuration := time.Duration(len(audio)+len(quiet)) * time.Second / 32000
	progress := newContinuousProgress(*turns, copyDuration)
	sent := make(chan sendResult, 1)
	go func() {
		report := sendInput(ctx, session, audio, quiet, *chunkSize, *mode, *turns, *expect, progress)
		sent <- report
		if report.err != nil {
			_ = session.Close()
		}
	}()

	seen := make(map[string]struct{})
	var lastText string
	var recvErr error
	var lastFinal bool
	for result, err := range session.Recv() {
		if err != nil {
			recvErr = err
			break
		}
		lastText, lastFinal = result.Text, result.IsFinal
		fmt.Printf("[final=%v] %s\n", result.IsFinal, result.Text)
		for _, utterance := range result.Utterances {
			if !utterance.Definite || strings.TrimSpace(utterance.Text) == "" {
				continue
			}
			key := fmt.Sprintf("%d:%d:%s", utterance.StartTime, utterance.EndTime, utterance.Text)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			progress.record(utterance)
			fmt.Printf("[definite start=%d end=%d] %s\n", utterance.StartTime, utterance.EndTime, utterance.Text)
		}
	}
	completedAt := time.Now()
	if recvErr != nil {
		cancel()
		_ = session.Close()
	}
	report := <-sent
	if recvErr != nil {
		return fmt.Errorf("receive result: %w", recvErr)
	}
	if report.err != nil {
		return report.err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !lastFinal || strings.TrimSpace(lastText) == "" || completedAt.Before(report.eosAt) {
		return fmt.Errorf("session ended without a non-empty final transcript after audio EOS")
	}
	if billed := session.AudioDuration(); billed+100*time.Millisecond < report.duration {
		return fmt.Errorf("provider processed %s of %s sent audio", billed, report.duration)
	}
	if *expect != "" {
		want := 1
		if *mode == "continuous" {
			want = *turns
		}
		if got := strings.Count(lastText, *expect); got < want {
			return fmt.Errorf("final transcript %q contains %q %d times, want at least %d", lastText, *expect, got, want)
		}
	}
	fmt.Printf("PASS mode=%s definite_utterances=%d final_latency=%s billed=%s sent=%s transcript=%q\n",
		*mode, len(seen), completedAt.Sub(report.eosAt).Round(time.Millisecond), session.AudioDuration(), report.duration.Round(time.Millisecond), lastText)
	return nil
}

func sendInput(ctx context.Context, session *doubaospeech.ASRV2Session, audio, quiet []byte, chunkSize int, mode string, turns int, expect string, progress *continuousProgress) sendResult {
	var sent time.Duration
	if mode == "ptt" {
		turns = 1
	}
	for turn := range turns {
		if err := sendPCM(ctx, session, audio, chunkSize, &sent); err != nil {
			return sendResult{err: err}
		}
		if mode == "continuous" {
			if err := sendPCM(ctx, session, quiet, chunkSize, &sent); err != nil {
				return sendResult{err: err}
			}
			if err := progress.check(turn, expect); err != nil {
				return sendResult{err: err}
			}
			fmt.Printf("continuous input %d finalized before audio EOS\n", turn+1)
		}
	}
	eosAt := time.Now()
	if err := session.SendAudio(ctx, nil, true); err != nil {
		return sendResult{err: fmt.Errorf("send audio EOS: %w", err)}
	}
	return sendResult{duration: sent, eosAt: eosAt}
}

// sendPCM sends packets at the PCM audio's wall-clock cadence.
func sendPCM(ctx context.Context, session *doubaospeech.ASRV2Session, audio []byte, chunkSize int, sent *time.Duration) error {
	started := time.Now()
	for offset := 0; offset < len(audio); offset += chunkSize {
		end := min(offset+chunkSize, len(audio))
		if err := session.SendAudio(ctx, audio[offset:end], false); err != nil {
			return fmt.Errorf("send audio: %w", err)
		}
		*sent += time.Duration(end-offset) * time.Second / 32000
		deadline := started.Add(time.Duration(end) * time.Second / 32000)
		timer := time.NewTimer(time.Until(deadline))
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
	return nil
}

func isLFSPointerContent(data []byte) bool {
	const lfsPointerPrefix = "version https://git-lfs.github.com/spec/v1"
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte(lfsPointerPrefix))
}
