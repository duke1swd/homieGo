package homie

// test the publication of things.

import (
	"testing"
)

const testTopicBase = "testing"

func createTestDevice() *Device {
	d := NewDevice("test-device-0000", "Test Device 0")
	d.SetTopicBase(testTopicBase)
	return d
}

func TestPublication(t *testing.T) {
	getTestClient(t)
	cleanMqtt(t, testTopicBase)
	//d := createTestDevice()
}
