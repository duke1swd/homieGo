package homie

//
// This file contains code to interface with the paho mqtt client.
//

import (
	"github.com/eclipse/paho.mqtt.golang"
	"log"
	"time"
)

var clientToDevice map[mqtt.Client]Device

func init() {
	clientToDevice = make(map[mqtt.Client]Device)
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
		panic("Connection Lost Handler cannot find device in map")
	}
	d.connectChannel <- true
}

func (d Device) mqttSetup() {
	if d.connected {
		panic("called setup on a connected device")
	}

	options := mqtt.NewClientOptions()
	options.SetCleanSession(false)
	options.SetClientID(mqttClientIDPrefix + "-" + d.id)
	options.SetAutoReconnect(true)
	options.SetConnectRetry(true)
	options.SetConnectRetryInterval(time.Minute)
	options.SetConnectionLostHandler(connectionLostHandler)
	options.SetOnConnectHandler(connectionFound)
	options.SetOrderMatters(false)
	options.SetWill(d.topic("$state"), "lost", 1, true)

	d.client = mqtt.NewClient(options)
	d.client.Connect()
}

// Check for publish errors. If found, log them.
// Token t has already been waited for.
func (d Device) tokenFinalize(t *mqtt.Token) {
	e := (*t).Error()
	log.Printf("Publish error %v\n", e)
}