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

	nodeMessages = map[string]string{
		"testing/test-device-0000/a-node/$type":       "test",
		"testing/test-device-0000/another-node/$name": "Name another-node",
		"testing/test-device-0000/another-node/$type": "test",
		"testing/test-device-0000/$nodes":             "a-node,another-node",
		"testing/test-device-0000/a-node/$name":       "Name a-node",
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

func myTestHandler(d *Device, n *Node, p *Property, a string) {
}

func createTestNode(d *Device, id string) {
	d.NewNode(id, "Name "+id, "test", myTestHandler)
}

func TestPublication(t *testing.T) {
	getTestClient(t)
	cleanMqtt(t)
	d := createTestDevice()
	createTestNode(d, "a-node")
	createTestNode(d, "another-node")

	// Run for 1 second
	c, cfl := context.WithTimeout(context.Background(), time.Second*time.Duration(1))
	d.RunWithContext(c)
	cfl()

	stuff := verifyMqtt(t, deviceMessages, nodeMessages)

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
