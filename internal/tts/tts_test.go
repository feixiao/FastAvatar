package tts

import (
	"context"
	"testing"
)

func TestMockSynthesizer(t *testing.T) {
	mock := NewMockSynthesizer()
	ctx := context.Background()

	mock.SetVoice("Sin-Ji")
	mock.SetRate(190)

	if mock.GetVoice() != "Sin-Ji" || mock.GetRate() != 190 {
		t.Fatalf("unexpected voice config: %s / %d", mock.GetVoice(), mock.GetRate())
	}

	if err := mock.Speak(ctx, "测试文本"); err != nil {
		t.Fatalf("Speak failed: %v", err)
	}

	if len(mock.Spoken) != 1 || mock.Spoken[0] != "测试文本" {
		t.Fatalf("unexpected spoken content: %v", mock.Spoken)
	}

	records := mock.GetRecords()
	if len(records) != 1 || records[0].Voice != "Sin-Ji" || records[0].Rate != 190 {
		t.Fatalf("unexpected record: %+v", records)
	}

	mock.Stop()
	if !mock.IsStopped() {
		t.Fatalf("expected stopped to be true")
	}
}

func TestMacSayCancel(t *testing.T) {
	synthesizer := NewMacSaySynthesizer("Ting-Ting", 180)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即刻取消

	_ = synthesizer.Speak(ctx, "这是一段应该被取消的长文本")
	synthesizer.Stop()
}
