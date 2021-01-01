package homie

// test the publication of things.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	deviceMessages = map[string]string{
		"testing/test-device-%04d/$state":          "disconnected",
		"testing/test-device-%04d/$homie":          "4.0.0",
		"testing/test-device-%04d/$name":           "Test Device 0",
		"testing/test-device-%04d/$implementation": "homieGo 0.1.0",
		"testing/test-device-%04d/$stats/uptime":   "*",
		"testing/test-device-%04d/$fw/name":        "unknown",
		"testing/test-device-%04d/$extensions":     "org.homie.legacy-stats:0.1.1:[4.x],org.homie.legacy-firmware:0.1.1:[4.x]",
		"testing/test-device-%04d/$stats/interval": "60",
		"testing/test-device-%04d/$fw/version":     "unknown",
	}

	nodeMessages = map[string]string{
		"testing/test-device-%04d/a-node/$type":       "test",
		"testing/test-device-%04d/another-node/$name": "Name another-node",
		"testing/test-device-%04d/another-node/$type": "test",
		"testing/test-device-%04d/$nodes":             "a-node,another-node",
		"testing/test-device-%04d/a-node/$name":       "Name a-node",
	}

	testTopicBase string
	deviceCounter int
)

func init() {
	testTopicBase = "testing"
	deviceCounter = 0
}

func createTestDevice() *Device {
	deviceCounter += 1
	d := NewDevice(fmt.Sprintf("test-device-%04d", deviceCounter), "Test Device 0")
	d.SetTopicBase(testTopicBase)
	return d
}

func myTestHandler(d *Device, n *Node, p *Property, a string) bool {
	return true
}

func createTestNode(d *Device, id string) {
	d.NewNode(id, "Name "+id, "test", myTestHandler)
}

func dmSub(iMes map[string]string, c int) map[string]string {
	r := make(map[string]string)

	for k, v := range iMes {
		r[fmt.Sprintf(k, c)] = v
	}
	return r
}

func TestPublication(t *testing.T) {
	getTestClient(t)
	cleanMqtt(t)
	d := createTestDevice()
	createTestNode(d, "a-node")
	createTestNode(d, "another-node")

	// Run for 1 second
	c, cfl := context.WithTimeout(context.Background(), time.Second*time.Duration(1))
	d.RunWithContext(c, make(chan bool, 1))
	cfl()

	stuff := verifyMqtt(t, dmSub(deviceMessages, deviceCounter), dmSub(nodeMessages, deviceCounter))

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
