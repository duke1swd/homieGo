package homie

//
// This file contains code to interface with the paho mqtt client.
//

import (
	"time"
	"github.com/eclipse/paho.mqtt.golang"
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
	options.SetClientID(clientIDPrefix + "-" + d.id)
	options.SetAutoConnect(true)
	options.SetConnectRetry(true)
	options.SetConnectRetryInterval(time.Minute)
	options.SetConnectionLostHandler(connectionLostHandler)
	options.SetOnConnectHandler(connectionFound)
	options.SetOrderMatters(false)
	options.SetWill(d.topic("$state"), "lost", 1, true)

	d.client = mqtt.NewClient(options)
	d.client.Connect()
}
