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
	allTopics         map[string]string
	testClient        mqtt.Client
	timeoutChannel    chan int = make(chan int)
	clientInitialized bool     = false
)

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
	if clientInitialized {
		return
	}
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("fw-test")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f1)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Errorf("MQTT Connect`failed: %v", token.Error())
	}
	testClient = c
	clientInitialized = true
}

// get all the persistent messages and build a map of everything we know about everybody
func getMqttStuff(t *testing.T) {
	c := testClient
	allTopics = make(map[string]string)

	subscription := testTopicBase + "/#"

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

func cleanMqtt(t *testing.T) {
	getMqttStuff(t)

	for topic, _ := range allTopics {
		token := testClient.Publish(topic, 1, true, "")
		if token.Wait() && token.Error() != nil {
			t.Errorf("cleaning topic %s yields publication error %v", topic, token.Error())
		}
	}
}

func getAllMqtt(t *testing.T) map[string]string {
	getMqttStuff(t)
	return allTopics
}

func verifyMqtt(t *testing.T, messageMaps ...map[string]string) map[string]string {
	getMqttStuff(t)

	// Look for the messages we expect
	for _, messages := range messageMaps {
		for k, v := range messages {
			if v2, ok := allTopics[k]; ok {
				if v != "*" && v != v2 {
					t.Errorf("For topic %s expected value \"%s\" found value \"%s\"", k, v, v2)
				}
			} else {
				t.Errorf("Did not find topic %s", k)
			}
		}
	}

	// Look for messages we did not expect
seekLoop:
	for k, v := range allTopics {
		for _, messages := range messageMaps {
			if _, ok := messages[k]; ok {
				continue seekLoop
			}
		}
		t.Errorf("Did not expect %s: %s", k, v)
	}

	return allTopics
}
