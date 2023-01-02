package homie

//
// This file contains code to interface with the paho mqtt client.
//

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"log"
	"time"
)

func (d *Device) mqttSetup() {
	if d.connected {
		panic("called setup on a connected device")
	}

	// re-use the existing clientOptions and client if we are reinitializing an existing device
	if d.clientOptions == nil {
		d.clientOptions = mqtt.NewClientOptions()
	}

	// d.clientOptions.SetPingTimeout(1 * time.Second)	// default of 10 seconds is fine

	d.clientOptions.SetKeepAlive(60 * time.Second)
	d.clientOptions.SetCleanSession(true) // XXX	not sure which I want, but this way works, 'false' doesn't
	d.clientOptions.AddBroker(d.mqttBroker)
	d.clientOptions.SetClientID(mqttClientIDPrefix + "-" + d.id)
	d.clientOptions.SetAutoReconnect(true)
	d.clientOptions.SetConnectRetry(true)
	d.clientOptions.SetConnectRetryInterval(time.Minute)
	d.clientOptions.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		d.connected = false
		d.connectChannel <- false
	})
	d.clientOptions.SetOnConnectHandler(func(c mqtt.Client) { d.connectChannel <- true })
	d.clientOptions.SetOrderMatters(false)
	d.clientOptions.SetWill(d.topic("$state"), "lost", 1, true)

	if d.client == nil {
		d.client = mqtt.NewClient(d.clientOptions)
	}

	token := d.client.Connect()
	// I don't know if token.Wait() will block, so ...
	go func(t mqtt.Token) {
		t.Wait()
		if t.Error() != nil {
			panic(fmt.Sprintf("Mqtt connect fails with error %v", t.Error()))
		}
	}(token)
}

// Check for publish errors. If found, log them.
// Token t has already been waited for.
func (d *Device) tokenFinalize(t *mqtt.Token) {
	if e := (*t).Error(); e != nil {
		log.Printf("Publish error %v\n", e)
	}
}
