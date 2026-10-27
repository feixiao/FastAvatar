package dispatcher

import (
	"testing"
	"time"
)

func TestDispatcher_EvaluationRoutes(t *testing.T) {
	d := NewFastDispatcher()

	cases := []struct {
		name       string
		text       string
		consented  bool
		online     bool
		wantTarget RouteTarget
		wantIntent string
	}{
		{
			name:       "persona switch",
			text:       "切换成伴侣模式",
			consented:  true,
			online:     true,
			wantTarget: RouteLocalPersona,
			wantIntent: "persona.switch",
		},
		{
			name:       "memory command",
			text:       "记住我喜欢喝冰美式",
			consented:  true,
			online:     true,
			wantTarget: RouteLocalMemory,
			wantIntent: "memory.command",
		},
		{
			name:       "mock vehicle - climate",
			text:       "把空调调到24度",
			consented:  true,
			online:     true,
			wantTarget: RouteMockVehicle,
			wantIntent: "climate.set_temperature",
		},
		{
			name:       "mock vehicle stays local even offline",
			text:       "打开全车车窗",
			consented:  false,
			online:     false,
			wantTarget: RouteMockVehicle,
			wantIntent: "window.control",
		},
		{
			name:       "open-ended not consented",
			text:       "今天有什么新鲜事",
			consented:  false,
			online:     true,
			wantTarget: RouteLocalOnly,
			wantIntent: "dialog.open_ended",
		},
		{
			name:       "open-ended offline",
			text:       "讲个睡前故事吧",
			consented:  true,
			online:     false,
			wantTarget: RouteOffline,
			wantIntent: "dialog.open_ended",
		},
		{
			name:       "open-ended cloud ready",
			text:       "为什么天空是蓝色的",
			consented:  true,
			online:     true,
			wantTarget: RouteCloudChat,
			wantIntent: "dialog.open_ended",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := d.Evaluate(tc.text, tc.consented, tc.online)
			if res.Target != tc.wantTarget {
				t.Errorf("text: %s, wantTarget: %v, got: %v", tc.text, tc.wantTarget, res.Target)
			}
			if res.IntentName != tc.wantIntent {
				t.Errorf("text: %s, wantIntent: %v, got: %v", tc.text, tc.wantIntent, res.IntentName)
			}
			if res.Latency > 5*time.Millisecond {
				t.Errorf("latency too high: %v", res.Latency)
			}
		})
	}
}

func TestDispatcher_BenchmarkLatency(t *testing.T) {
	d := NewFastDispatcher()
	text := "主驾空调温度调至25度"

	iterations := 10000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		res := d.Evaluate(text, true, true)
		if res.Target != RouteMockVehicle {
			t.Fatalf("unexpected route: %v", res.Target)
		}
	}
	elapsed := time.Since(start)
	avg := elapsed / time.Duration(iterations)

	t.Logf("Completed %d dispatches in %v (avg: %v per evaluation)", iterations, elapsed, avg)
	if avg > 100*time.Microsecond {
		t.Errorf("average latency exceeded 100us: %v", avg)
	}
}
