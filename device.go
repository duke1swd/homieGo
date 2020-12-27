package homie

import (
	"time"
)

func NewDevice(id, name string) *Device {
	var device Device

	id = validate(id, false)

	if _, ok := devices[id]; ok {
		panic("Duplicate device id: " + id)
	}

	device.id = id
	device.protocol = "4.0.0"
	device.name = name
	device.state = "init"
	device.implementation = "homieGo 0.1.0"
	device.nodes = make(map[string]*Node)

	device.extensions = "org.homie.legacy-stats:0.1.1:[4.x],org.homie.legacy-firmware:0.1.1:[4.x]"
	device.statsInterval = time.Duration(60) * time.Second
	device.fwName = "unknown"
	device.fwVersion = "unknown"

	device.configDone = false

	device.publishChannel = make(chan PropertyMessage, 100)
	device.period = time.Second / time.Duration(4)

	devices[id] = &device

	return &device
}

func (d *Device) SetGlobalHandler(handler func(d *Device, n Node, p Property, value string)) {
	d.globalHandler = handler
}

func (d *Device) SetBroadcastHandler(handler func(d *Device, level, value string)) {
	d.broadcastHandler = handler
}

func (d *Device) SetLoop(handler func(d *Device)) {
	d.loop = handler
}

func (d *Device) IsConnected() bool {
	return d.connected
}

// Publish everything about this device.
// This is done on connection to (and reconnection to) the mqtt broker
func (d *Device) publishState() {
}

func (d *Device) setLoopPeriod(period time.Duration) {
	if d.configDone {
		panic("Cannot change loop period after calling Run() for device " + d.id)
	}

	d.period = period
}

// Run the control loop
// All error conditions return by panic.
// No normaal return
func (d *Device) Run() {
	var (
		ticker *time.Ticker
	)

	d.configDone = true
	d.connected = false
	if d.period > 0 {
		ticker = time.NewTicker(d.period)
	}

	for {
		// process reconnections
		if !d.connected {
			// TODO
			// connect
		}

		// Call the user's loop function
		if d.loop != nil {
			d.loop(d)
		}

		// Drain the publish channel
		for {
			// non-blocking look for an incoming publish message
			select {
			case message := <-d.publishChannel:
				message.publish()
			default:
				break
			}
		}

		// now sleep for awhile if necessary
		if d.period > 0 {
			for {
				select {
				case message := <-d.publishChannel:
					message.publish()
				case _ = <-ticker.C:
					break
				}
			}
		}
	}
}
