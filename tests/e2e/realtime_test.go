package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

const (
	realtimeE2ETurnTimeout   = 45 * time.Second
	realtimeE2EQuery         = "请用一句话介绍你自己。"
	realtimeE2EO20Speaker    = "zh_female_vv_jupiter_bigtts"
	realtimeE2ESC20Speaker   = "saturn_zh_female_cancan_tob"
	realtimeE2EInstructions  = "你是一个简洁的中文助手，每次只用一句话回答。"
	realtimeE2EEventLogLimit = 64
	realtimeE2ESettleWindow  = 3 * time.Second
)

// TestRealtimeOutputModalities verifies the undocumented
// dialog.extra.output_modalities StartSession field against the live realtime
// dialogue service. A text-only session must deliver ChatResponse text and
// ChatEnded without any TTSResponse audio; a text+audio session must still
// deliver TTS audio. It is opt-in because it uses paid credentials.
func TestRealtimeOutputModalities(t *testing.T) {
	loadE2EEnv(t)
	if os.Getenv("DOUBAO_RUN_LIVE") != "1" {
		t.Skip("set DOUBAO_RUN_LIVE=1 in tests/e2e/.env to run the realtime E2E test")
	}

	appID := firstEnv("DOUBAO_REALTIME_APP_ID", "DOUBAO_APP_ID")
	if appID == "" {
		t.Fatal("DOUBAO_REALTIME_APP_ID or DOUBAO_APP_ID is required")
	}
	apiKey := firstEnv("DOUBAO_REALTIME_API_KEY", "DOUBAO_API_KEY")
	if apiKey == "" {
		t.Fatal("DOUBAO_REALTIME_API_KEY or DOUBAO_API_KEY is required")
	}
	resourceID := firstEnv("DOUBAO_REALTIME_RESOURCE_ID")
	if resourceID == "" {
		resourceID = doubaospeech.ResourceRealtime
	}
	client := doubaospeech.NewClient(
		appID,
		doubaospeech.WithAPIKey(apiKey),
		doubaospeech.WithResourceID(resourceID),
		doubaospeech.WithUserID("realtime-e2e"),
	)

	models := []struct {
		name    string
		model   doubaospeech.RealtimeModelVersion
		speaker string
	}{
		{name: "O20", model: doubaospeech.RealtimeModelO20, speaker: realtimeE2EO20Speaker},
		{name: "SC20", model: doubaospeech.RealtimeModelSC20, speaker: realtimeE2ESC20Speaker},
	}
	for _, model := range models {
		if speaker := firstEnv("DOUBAO_REALTIME_" + model.name + "_SPEAKER"); speaker != "" {
			model.speaker = speaker
		}
		t.Run(model.name, func(t *testing.T) {
			t.Run("text", func(t *testing.T) {
				turn := runRealtimeTextTurn(t, client, model.model, model.speaker, []doubaospeech.RealtimeOutputModality{
					doubaospeech.RealtimeOutputModalityText,
				})
				if turn.audioBytes != 0 || turn.counts[doubaospeech.EventTTSAudioData] != 0 {
					t.Fatalf("text-only session returned %d TTSResponse events with %d audio bytes; events=%s",
						turn.counts[doubaospeech.EventTTSAudioData], turn.audioBytes, turn.sequence())
				}
				// Downstream turn lifecycles rely on the text-only turn ending at
				// ChatEnded: no TTS sentence or TTSEnded events are sent.
				for _, event := range []doubaospeech.RealtimeEventType{
					doubaospeech.EventTTSStarted,
					doubaospeech.EventTTSSegmentEnd,
					doubaospeech.EventTTSFinished,
				} {
					if turn.counts[event] != 0 {
						t.Fatalf("text-only session returned %d events of type %d; events=%s", turn.counts[event], event, turn.sequence())
					}
				}
				if !turn.chatEndedFinal {
					t.Fatalf("text-only ChatEnded IsFinal = false, want true; events=%s", turn.sequence())
				}
			})
			t.Run("text_audio", func(t *testing.T) {
				turn := runRealtimeTextTurn(t, client, model.model, model.speaker, []doubaospeech.RealtimeOutputModality{
					doubaospeech.RealtimeOutputModalityText,
					doubaospeech.RealtimeOutputModalityAudio,
				})
				if turn.audioBytes == 0 {
					t.Fatalf("text+audio session returned no TTSResponse audio; events=%s", turn.sequence())
				}
				if turn.counts[doubaospeech.EventTTSFinished] == 0 {
					t.Fatalf("text+audio session returned no TTSEnded; events=%s", turn.sequence())
				}
			})
		})
	}
}

type realtimeE2ETurn struct {
	text           string
	audioBytes     int
	chatEndedFinal bool
	counts         map[doubaospeech.RealtimeEventType]int
	events         []doubaospeech.RealtimeEventType
}

func (turn realtimeE2ETurn) sequence() string {
	parts := make([]string, 0, len(turn.events))
	for i, event := range turn.events {
		if i > 0 && event == turn.events[i-1] {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d", event))
	}
	return strings.Join(parts, ",")
}

// runRealtimeTextTurn opens a text-input session with the given output
// modalities, sends one ChatTextQuery, and collects events until the chat
// reply and any TTS lifecycle for that turn have settled.
func runRealtimeTextTurn(
	t *testing.T,
	client *doubaospeech.Client,
	model doubaospeech.RealtimeModelVersion,
	speaker string,
	modalities []doubaospeech.RealtimeOutputModality,
) realtimeE2ETurn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), realtimeE2ETurnTimeout)
	defer cancel()

	cfg := doubaospeech.DefaultRealtimeConfig()
	cfg.Model = model
	cfg.InputMode = doubaospeech.RealtimeInputModeText
	cfg.TTS.Speaker = speaker
	cfg.Instructions = realtimeE2EInstructions
	cfg.Dialog.Extra = &doubaospeech.RealtimeDialogExtra{OutputModalities: modalities}

	session, err := client.Realtime.OpenSession(ctx, &cfg)
	if err != nil {
		t.Fatalf("open realtime session model=%s modalities=%v: %v", model, modalities, err)
	}
	defer session.Close()

	if err := session.SendUserMessage(ctx, realtimeE2EQuery); err != nil {
		t.Fatalf("send realtime text query: %v", err)
	}

	turn := realtimeE2ETurn{counts: map[doubaospeech.RealtimeEventType]int{}}
	var text strings.Builder
	chatEnded := false
	for !chatEnded || turn.counts[doubaospeech.EventTTSFinished] == 0 {
		// Text-only sessions never send TTSEnded, so once ChatEnded arrives
		// wait only a short settle window for late TTS events.
		recvCtx, recvCancel := ctx, context.CancelFunc(func() {})
		if chatEnded {
			recvCtx, recvCancel = context.WithTimeout(ctx, realtimeE2ESettleWindow)
		}
		evt, err := session.RecvEvent(recvCtx)
		recvCancel()
		if err != nil {
			if chatEnded && ctx.Err() == nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF)) {
				break
			}
			t.Fatalf("receive realtime event model=%s modalities=%v: %v; events=%s", model, modalities, err, turn.sequence())
		}
		if evt == nil {
			continue
		}
		turn.counts[evt.Type]++
		if len(turn.events) < realtimeE2EEventLogLimit {
			turn.events = append(turn.events, evt.Type)
		}
		switch evt.Type {
		case doubaospeech.EventChatResponse:
			text.WriteString(evt.Text)
		case doubaospeech.EventTTSAudioData:
			turn.audioBytes += len(evt.Audio)
		case doubaospeech.EventChatEnded:
			chatEnded = true
			turn.chatEndedFinal = evt.IsFinal
		case doubaospeech.EventDialogCommonError:
			t.Fatalf("realtime dialog error model=%s modalities=%v: %v", model, modalities, evt.Error)
		}
	}
	turn.text = strings.TrimSpace(text.String())

	if err := session.FinishSession(ctx); err != nil {
		t.Logf("finish realtime session: %v", err)
	}

	t.Logf(
		"realtime model=%s modalities=%v text=%q audio_bytes=%d chat_response=%d chat_ended=%d tts_sentence_start=%d tts_sentence_end=%d tts_response=%d tts_ended=%d usage=%d events=%s",
		model,
		modalities,
		turn.text,
		turn.audioBytes,
		turn.counts[doubaospeech.EventChatResponse],
		turn.counts[doubaospeech.EventChatEnded],
		turn.counts[doubaospeech.EventTTSStarted],
		turn.counts[doubaospeech.EventTTSSegmentEnd],
		turn.counts[doubaospeech.EventTTSAudioData],
		turn.counts[doubaospeech.EventTTSFinished],
		turn.counts[doubaospeech.EventUsageResponse],
		turn.sequence(),
	)
	if turn.counts[doubaospeech.EventChatResponse] == 0 || turn.text == "" {
		t.Fatalf("realtime session returned no ChatResponse text; events=%s", turn.sequence())
	}
	if turn.counts[doubaospeech.EventChatEnded] == 0 {
		t.Fatalf("realtime session returned no ChatEnded; events=%s", turn.sequence())
	}
	return turn
}
