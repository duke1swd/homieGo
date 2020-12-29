package homie

//
// This file contains code to interface with the paho mqtt client.
// Used by test code only
//

import (
	"github.com/eclipse/paho.mqtt.golang"
	"strings"
	"testing"
	"time"
)

var (
	allTopics      map[string]string
	testClient     mqtt.Client
	timeoutChannel chan int = make(chan int)
)

func cleanMqtt(t *testing.T, topicBase string) {
	getMqttStuff(t, topicBase)
}

func getAllMqtt(t *testing.T, topicBase string) map[string]string {
	getMqttStuff(t, topicBase)
	return allTopics
}

var f1 mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Ignore broadcast messages
	if strings.Contains(topic, "$broadcast") {
		return
	}

	if strings.Contains(topic, "firmware") {
		payload = "(suppressed)"
	}

	// tell the world we are still working
	timeoutChannel <- 0

	allTopics[topic] = payload
}

func getTestClient(t *testing.T) {
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("fw-test")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f1)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Errorf("MQTT Connect`failed: %v", token.Error())
	}
	testClient = c
}

// get all the persistent messages and build a map of everything we know about everybody
func getMqttStuff(t *testing.T, topicBase string) {
	c := testClient

	subscription := topicBase + "/#"

	if token := c.Subscribe(subscription, 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("MQTT Subscribe failed: %v", token.Error())
	}

	// Wait for 1 second after last message is received
	timer := time.NewTimer(time.Second)
waitLoop:
	for {
		select {
		case _ = <-timeoutChannel:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(time.Second)
		case _ = <-timer.C:
			break waitLoop
		}
	}

	if token := c.Unsubscribe(subscription); token.Wait() && token.Error() != nil {
		t.Errorf("MQTT Unsubscribe failed: %v", token.Error())
	}
}
