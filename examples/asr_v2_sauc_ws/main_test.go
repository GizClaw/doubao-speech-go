package main

import (
	"strings"
	"testing"
	"time"

	doubaospeech "github.com/GizClaw/doubao-speech-go"
)

func TestContinuousExpectedTextBelongsToEachCopy(t *testing.T) {
	progress := newContinuousProgress(2, 5*time.Second, 0)
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 0, EndTime: 2000, Text: "target target"})
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 6000, EndTime: 7000, Text: "other"})
	if err := progress.check(0, "target"); err != nil {
		t.Fatal(err)
	}
	if err := progress.check(1, "target"); err == nil || !strings.Contains(err.Error(), "input 2") {
		t.Fatalf("second copy check = %v, want a missing-target error despite two matches in the first copy", err)
	}
}

func TestContinuousCopyCollectsOnlyItsFinalizedUtterances(t *testing.T) {
	progress := newContinuousProgress(2, 5*time.Second, 0)
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 0, EndTime: 2000, Text: "hello "})
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 2000, EndTime: 4000, Text: "world"})
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 6000, EndTime: 9000, Text: "hello world"})
	for copy := range 2 {
		if err := progress.check(copy, "hello world"); err != nil {
			t.Fatal(err)
		}
	}

	spanning := newContinuousProgress(2, 5*time.Second, 0)
	spanning.record(doubaospeech.ASRV2Utterance{StartTime: 4000, EndTime: 6000, Text: "hello world"})
	for copy := range 2 {
		if err := spanning.check(copy, "hello world"); err == nil {
			t.Fatalf("utterance spanning input copies satisfied copy %d", copy+1)
		}
	}
}

func TestContinuousCopyAllowsVADPrerollOnlyInPrecedingSilence(t *testing.T) {
	progress := newContinuousProgress(2, 4352*time.Millisecond, 2*time.Second)
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 0, EndTime: 2252, Text: "target"})
	progress.record(doubaospeech.ASRV2Utterance{StartTime: 4222, EndTime: 6652, Text: "target"})
	for copy := range 2 {
		if err := progress.check(copy, "target"); err != nil {
			t.Fatal(err)
		}
	}

	spanning := newContinuousProgress(2, 4352*time.Millisecond, 2*time.Second)
	spanning.record(doubaospeech.ASRV2Utterance{StartTime: 2300, EndTime: 6652, Text: "target"})
	if err := spanning.check(1, "target"); err == nil {
		t.Fatal("utterance including the preceding copy's speech satisfied the next copy")
	}
}
