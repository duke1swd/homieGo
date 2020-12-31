package homie

import (
	"github.com/eclipse/paho.mqtt.golang"
)

// Property methods
// All of the public methods can be called from an event handler.
// Actual publication is deferred out of the event handler.
// Publication happens in Device.Run().

func (p Property) Settable(handler func(d *Device, n *Node, p *Property, value string) bool) {
	p.settable = true
	p.handler = handler
}

func (p *Property) validateUnit(unit string) string {
	if _, ok := propertyUnits[unit]; !ok {
		panic("invalid unit " + unit + "for property " + p.id + " in node " + p.node.id)
	}
	return unit
}

func (p *Property) SetUnit(unit string) {
	if p.node.device.configDone {
		panic("Cannot setUnit on property " + p.id + " for node " + p.node.id + " to device " + p.node.device.name +
			" after calling device.Run()")
	}

	p.unit = p.validateUnit(unit)
}

func (p *Property) validateFormat(format string) string {
	return format
}

func (p *Property) SetFormat(format string) {
	if p.node.device.configDone {
		panic("Cannot setFormat on property " + p.id + " for node " + p.node.id + " to device " + p.node.device.name +
			" after calling device.Run()")
	}

	p.format = p.validateFormat(format)
}

func (p *Property) SetProperty() PropertyMessage {
	var m PropertyMessage

	m.Qos = 1
	m.Retained = true
	m.property = p

	return m
}

func (p *Property) topic(t string) string {
	return p.node.topic(p.id + "/" + t)
}

func (p *Property) publish(topic, payload string) {
	p.node.publish(p.id+"/"+topic, payload)
}

func (p *Property) processConnect() {
	var t string

	n := p.node

	p.publish("$name", p.name)

	switch p.dataType {
	case DtString:
		t = "string"
	case DtInteger:
		t = "integer"
	case DtFloat:
		t = "float"
	case DtBoolean:
		t = "boolean"
	case DtEnum:
		t = "enum"
	case DtColor:
		t = "color"
	}
	p.publish("$datatype", t)

	if len(p.format) > 0 {
		p.publish("$format", p.format)
	}

	if p.settable {
		p.publish("$settable", "true")
	}

	if len(p.unit) > 0 {
		p.publish("$unit", p.unit)
	}

	// Finally spit out the value of this property.
	n.publish(p.id, p.value)

	// Is this property settable?  If so, subscribe to the set message.
	d := n.device
	d.client.Subscribe(p.topic("set"), 1, func(c mqtt.Client, msg mqtt.Message) { p.setEvent(string(msg.Payload())) })
	// Also subscribe to the value itself, to get the initial value
	valueTopic := p.node.topic(p.id)
	d.client.Subscribe(valueTopic, 1, func(c mqtt.Client, msg mqtt.Message) {
		if !p.node.device.configDone {
			p.setEvent(string(msg.Payload()))
		}
	})

	// When we are done configuring, this fn will be called to unsubscribe the base value subscription
	d.unsubscribes = append(d.unsubscribes, func() {
		t := d.client.Unsubscribe(valueTopic)
		d.tokenChannel <- &t
	})
}

// When a "set" message is received, this thread executes in some random go routine context.
func (p *Property) setEvent(value string) {
	n := p.node
	d := n.device

	if d.globalHandler != nil && d.globalHandler(d, n, p, value) {
		return
	}

	if n.handler != nil && n.handler(d, n, p, value) {
		return
	}

	if p.handler != nil {
		p.handler(d, n, p, value)
	}
}

func (m PropertyMessage) validateValue(value string) error {
	// TODO add checking
	return nil
}

// Returns an error if the property's value is wrong format, unit, or whatever.
// These errors are warnings only.
func (m PropertyMessage) Send(value string) error {
	m.property.value = value
	err := m.validateValue(value)
	if m.property.node.device.configDone {
		m.property.node.device.publishChannel <- m
	}
	return err
}

// Called by Device.Run() to do the actual publication of a new property value.
func (m PropertyMessage) publish() {
	n := m.property.node
	d := n.device
	token := d.client.Publish(n.topic(m.property.id), m.Qos, m.Retained, m.property.value)
	d.tokenChannel <- &token
}
