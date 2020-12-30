package homie

// test the publication of things.

import (
	"context"
	"testing"
	"time"
)

func init() {
	testTopicBase = "testing"
}

func createTestDevice() *Device {
	d := NewDevice("test_device_0000", "Test Device 0")
	d.SetTopicBase(testTopicBase)
	return d
}

func TestPublication(t *testing.T) {
	getTestClient(t)
	cleanMqtt(t)
	d := createTestDevice()

	// Run for 1 second
	c, cfl := context.WithTimeout(context.Background(), time.Second)
	d.RunWithContext(c)
	cfl()

	verifyMqtt(t, map[string]string{
		"testing/test_device_0000/$state": "ready",
	})
	cleanMqtt(t)
}
