package homie

// test the publication of things.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	deviceMessages = map[string]string{
		"testing/test-device-0000/$state":          "ready",
		"testing/test-device-0000/$homie":          "4.0.0",
		"testing/test-device-0000/$name":           "Test Device 0",
		"testing/test-device-0000/$implementation": "homieGo 0.1.0",
		"testing/test-device-0000/$stats/uptime":   "*",
		"testing/test-device-0000/$fw/name":        "unknown",
		"testing/test-device-0000/$extensions":     "org.homie.legacy-stats:0.1.1:[4.x],org.homie.legacy-firmware:0.1.1:[4.x]",
		"testing/test-device-0000/$stats/interval": "60",
		"testing/test-device-0000/$fw/version":     "unknown",
	}
)

func init() {
	testTopicBase = "testing"
}

func createTestDevice() *Device {
	d := NewDevice("test-device-0000", "Test Device 0")
	d.SetTopicBase(testTopicBase)
	return d
}

func TestPublication(t *testing.T) {
	getTestClient(t)
	cleanMqtt(t)
	d := createTestDevice()

	// Run for 1 second
	c, cfl := context.WithTimeout(context.Background(), time.Second*time.Duration(1))
	d.RunWithContext(c)
	cfl()

	stuff := verifyMqtt(t, deviceMessages)

	// check up time
	for topic, payload := range stuff {
		if strings.Contains(topic, "uptime") {
			if u, err := strconv.Atoi(payload); err != nil || u > 2 {
				t.Errorf("Uptime more than 2 seconds: %s", payload)
			}
		}
	}
	cleanMqtt(t)
}
