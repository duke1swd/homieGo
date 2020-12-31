package homie

import (
	"context"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"strconv"
	"strings"
	"time"
)

func NewDevice(id, name string) *Device {
	var device Device

	id = validate(id, false)

	if _, ok := devices[id]; ok {
		panic("Duplicate device id: " + id)
	}

	device.id = id
	device.configDone = false
	device.protocol = "4.0.0"
	device.name = name
	device.state = "init"
	device.implementation = "homieGo 0.1.0"
	device.nodes = make(map[string]*Node)

	device.extensions = "org.homie.legacy-stats:0.1.1:[4.x],org.homie.legacy-firmware:0.1.1:[4.x]"
	device.statsInterval = time.Duration(60) * time.Second
	device.statsBootTime = time.Now()
	device.fwName = "unknown"
	device.fwVersion = "unknown"
	device.topicBase = defaultTopicBase

	device.period = time.Second / time.Duration(4)

	device.publishChannel = make(chan PropertyMessage, 100)
	device.connectChannel = make(chan bool, 16)
	device.tokenChannel = make(chan *mqtt.Token, 256)
	device.globalHandler = nil
	device.broadcastHandler = nil

	device.clientOptions = nil
	device.client = nil

	devices[id] = &device

	return &device
}

func (d *Device) SetGlobalHandler(handler func(d *Device, n *Node, p *Property, value string) bool) {
	d.globalHandler = handler
}

func (d *Device) SetBroadcastHandler(handler func(d *Device, level, value string)) {
	d.broadcastHandler = handler
	if d.connected {
		d.subscribeToBroadcasts()
	}
}

func (d *Device) SetLoop(handler func(d *Device)) {
	d.loop = handler
}

func (d *Device) IsConnected() bool {
	return d.connected
}

func (d *Device) SetTopicBase(b string) {
	d.topicBase = validate(b, false)
}

func (d *Device) topic(t string) string {
	return d.topicBase + "/" + d.id + "/" + t
}

func (d *Device) publish(t, p string) {
	token := d.client.Publish(d.topic(t), 1, true, p)
	d.tokenChannel <- &token
}

func durationToSeconds(d time.Duration) string {
	n := int64(d) / int64(time.Second)
	return strconv.FormatInt(n, 10)
}

// wait for all publications
func (d *Device) waitAllPublications() {
tokenLoop:
	for {
		select {
		case t := <-d.tokenChannel:
			(*t).Wait()
			d.tokenFinalize(t)
		default:
			break tokenLoop
		}
	}
}

// Publish everything about this device.
// This is done on connection to (and reconnection to) the mqtt broker
func (d *Device) processConnect() {
	// Emit the required properties.
	d.publish("$state", "init")
	d.waitAllPublications() // force the "init" message out before any others.
	d.publish("$homie", d.protocol)
	d.publish("$name", d.name)
	d.publish("$extensions", d.extensions)
	d.publish("$implementation", d.implementation)

	// the extensions
	d.publish("$stats/interval", durationToSeconds(d.statsInterval))
	d.publish("$stats/uptime", durationToSeconds(time.Since(d.statsBootTime)))
	d.publish("$localip", d.localIP)
	d.publish("$mac", d.mac)
	d.publish("$fw/name", d.fwName)
	d.publish("$fw/version", d.fwVersion)

	if d.broadcastHandler != nil {
		d.subscribeToBroadcasts()
	}

	// Spit out the nodes
	if len(d.nodes) > 0 {
		s := ""
		for n, _ := range d.nodes {
			if len(s) > 0 {
				s = s + "," + n
			} else {
				s = n
			}
		}
		d.publish("$nodes", s)

		for _, n := range d.nodes {
			n.processConnect()
		}
	} else {
		d.publish("$nodes", "")
	}

	d.waitAllPublications()
	d.connected = true
	d.publish("$state", "ready")
}

func (d *Device) setLoopPeriod(period time.Duration) {
	if d.configDone {
		panic("Cannot change loop period after calling Run() for device " + d.id)
	}

	d.period = period
}

// subscribe to the broadcast channel.
// Blocks until broker acknowledges the subscription.
func (d *Device) subscribeToBroadcasts() {
	broadcastBase := d.topicBase + "/$broadcast/#"
	token := d.client.Subscribe(broadcastBase, 0,
		func(c mqtt.Client, m mqtt.Message) {
			if d.broadcastHandler != nil {
				topics := strings.SplitN(string(m.Topic()), "/", 3)
				if len(topics) == 3 {
					level := topics[2]
					d.broadcastHandler(d, level, string(m.Payload()))
				}
			}
		})
	token.Wait()
	if token.Error() != nil {
		panic(fmt.Sprintf("Error while subscribing to %s: %v", broadcastBase, token.Error()))
	}
}

// Run the control loop
// All error conditions return by panic.
// No normaal return
func (d *Device) Run() {
	d.RunWithContext(context.Background())
}

func (d *Device) RunWithContext(runContext context.Context) {
	var (
		ticker *time.Ticker
	)

	d.configDone = true
	d.connected = false
	d.mqttSetup()
	if d.period > 0 {
		ticker = time.NewTicker(d.period)
	}

runLoop:
	for {
		// Call the user's loop function
		if d.loop != nil {
			d.loop(d)
		}

		// Drain the channels
	drain:
		for {
			// Process any accumulated publish tokens
			// Do this without blocking.
			// If we are not connected, let the connecting goroutine
			// handle this.  Avoids simultaneous calls to d.tokenFinalize()
			if d.connected {
			tokenLoop:
				for {
					select {
					case t := <-d.tokenChannel:
						if (*t).WaitTimeout(time.Duration(0)) {
							d.tokenFinalize(t)
						} else {
							d.tokenChannel <- t
							break tokenLoop
						}
					default:
						break tokenLoop
					}
				}
			}
			// non blocking
			select {
			case message := <-d.publishChannel:
				// Message to publish?
				message.publish()

			case connected := <-d.connectChannel:
				// Change in connection status?
				if connected {
					go d.processConnect()
				}

			default:
				// If nothing to do, don't block
				break drain
			}
		}

		// now sleep for awhile if necessary
		// Keep checking for work while sleeping.
		// Note that there doesn't seem to be a good way of
		// processing publish tokens here.
		if d.period > 0 {
		sleepLoop:
			for {
				select {
				case message := <-d.publishChannel:
					message.publish()
				case connected := <-d.connectChannel:
					if connected {
						go d.processConnect()
					}
				case _ = <-ticker.C:
					break sleepLoop
				case <-runContext.Done():
					break sleepLoop
				}
			}
		}
		if runContext.Err() != nil {
			break runLoop
		}
	}

	// Come here to disconnect and exit
	d.publish("$state", "disconnected")
	d.waitAllPublications()
	d.clientOptions.UnsetWill()
	d.client.Disconnect(150) // disconnect in 0.15 seconds.
	d.configDone = false
}
