package homie

func NewDevice(id, name string) Device {
	var device Device

	id = validate(id, false)

	if _, ok := devices[id]; ok {
		panic("Duplicate device id: " + id)
	}

	device.id = id
	device.protocol = "4.0.1"
	device.name = name
	device.state = "init"
	device.nodes = make(map[string]Node)
	device.extensions = ""
	device.implementation = "homieGo 0.1.0"
	device.configDone = false

	devices[id] = device

	return device
}

// Publish everything about this device.
// This is done on connection to (and reconnection to) the mqtt broker
func (device Device) publishState() {
}

// Run the control loop
// All error conditions return by panic.
// No normaal return
func (device Device) Run() {
	device.configDone = true
}
