package homie

// test the publication of things.

import (
	"context"
	"testing"
	"time"
)

func TestBroadcast(t *testing.T) {
	broadcastLevel := ""
	broadcastValue := ""
	myLevel := "alarming!"
	myLevelValue := "now!"

	getTestClient(t)
	cleanMqtt(t)
	d := createTestDevice()
	createTestNode(d, "a-node")
	d.SetBroadcastHandler(func(d *Device, level, value string) {
		broadcastLevel = level
		broadcastValue = value
	})

	// Run for until cancelled
	waitChannel := make(chan bool, 1)
	c, cfl := context.WithCancel(context.Background())
	go d.RunWithContext(c, waitChannel)
	time.Sleep(time.Duration(25) * time.Millisecond)

	// send a broadcast
	token := d.client.Publish(testTopicBase+"/$broadcast/"+myLevel, 1, true, myLevelValue)
	token.Wait()
	if token.Error() != nil {
		t.Errorf("broadcast failed with error: %v", token.Error())
	}
	time.Sleep(time.Duration(25) * time.Millisecond)
	if broadcastLevel != myLevel {
		t.Errorf("broadcast level mismatch.  Expected \"%s\" got \"%s\"", myLevel, broadcastLevel)
	}
	if broadcastValue != myLevelValue {
		t.Errorf("broadcast value mismatch.  Expected \"%s\" got \"%s\"", myLevelValue, broadcastValue)
	}

	// terminate the run
	cfl()
	
	// wait for Run to come back
	for _ = range(waitChannel)  {
	}
	cleanMqtt(t)
}
