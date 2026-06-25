package doubaospeech

import "testing"

func TestClientCanonicalServices(t *testing.T) {
	client := NewClient("app-test", WithAPIKey("key-test"))

	if client.ASRV2 == nil {
		t.Fatalf("ASRV2 service is nil")
	}
	if client.TTSV2 == nil {
		t.Fatalf("TTSV2 service is nil")
	}
	if client.Realtime == nil {
		t.Fatalf("Realtime service is nil")
	}
	if client.RealtimeDuplex == nil {
		t.Fatalf("RealtimeDuplex service is nil")
	}
	if client.ASTTranslate == nil {
		t.Fatalf("ASTTranslate service is nil")
	}
	if client.AudioGeneration == nil {
		t.Fatalf("AudioGeneration service is nil")
	}
	if client.VoiceClone == nil {
		t.Fatalf("VoiceClone service is nil")
	}
}
