package system

import (
	"net"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (s *System) WatchNetworkQuality() {
	for {
		quality, ping := measureNetworkQuality()
		runtime.EventsEmit(s.ctx, "networkQuality", map[string]interface{}{
			"quality": quality,
			"ping":    ping.Milliseconds(),
		})
		time.Sleep(5 * time.Second)
	}
}

func measureNetworkQuality() (string, time.Duration) {
	start := time.Now()

	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 3*time.Second)
	if err != nil {
		return "offline", 0
	}
	_ = conn.Close()

	duration := time.Since(start)

	switch {
	case duration < 100*time.Millisecond:
		return "excellent", duration
	case duration < 300*time.Millisecond:
		return "good", duration
	case duration < 1000*time.Millisecond:
		return "average", duration
	default:
		return "poor", duration
	}
}
