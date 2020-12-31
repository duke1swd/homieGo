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

var clientToDevice map[mqtt.Client]*Device

func init() {
	clientToDevice = make(map[mqtt.Client]*Device)
}

func connectionLostHandler(client mqtt.Client, e error) {
	d, ok := clientToDevice[client]
	if !ok {
		panic("Connection Lost Handler cannot find device in map")
	}
	d.connected = false
	d.connectChannel <- false
}

func connectionFound(client mqtt.Client) {
	d, ok := clientToDevice[client]
	if !ok {
		panic("Connection Found Handler cannot find device in map")
	}
	d.connectChannel <- true
}

func (d *Device) mqttSetup() {
	if d.connected {
		panic("called setup on a connected device")
	}

	// re-use the existing clientOptions and client if we are reinitializing an existing device
	if d.clientOptions == nil {
		d.clientOptions = mqtt.NewClientOptions()
	}
	d.clientOptions.SetCleanSession(false)
	d.clientOptions.AddBroker("tcp://192.168.1.13:1883")
	d.clientOptions.SetClientID(mqttClientIDPrefix + "-" + d.id)
	d.clientOptions.SetAutoReconnect(true)
	d.clientOptions.SetConnectRetry(true)
	d.clientOptions.SetConnectRetryInterval(time.Minute)
	d.clientOptions.SetConnectionLostHandler(connectionLostHandler)
	d.clientOptions.SetOnConnectHandler(connectionFound)
	d.clientOptions.SetOrderMatters(false)
	d.clientOptions.SetWill(d.topic("$state"), "lost", 1, true)

	if d.client == nil {
		d.client = mqtt.NewClient(d.clientOptions)
		clientToDevice[d.client] = d
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
