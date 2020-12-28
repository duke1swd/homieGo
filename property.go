package homie

import ()

// Property methods

func (p Property) Settable(handler func(d Device, n Node, p Property, value string)) {
	p.settable = true
	p.handler = handler
}

func validateUnit(p Property, unit string) {
	if _, ok := propertyUnits[unit]; !ok {
		panic("invalid unit " + unit + "for property " + p.id + " in node " + p.node.id)
	}
}

func (p *Property) SetUnit(unit string) {
	if p.node.device.configDone {
		panic("Cannot setUnit on property " + p.id + " for node " + p.node.id + " to device " + p.node.device.name +
			" after calling device.Run()")
	}

	validateUnit(*p, unit)
	p.unit = unit
}

func validateFormat(p Property, format string) {
}

func (p *Property) SetFormat(format string) {
	if p.node.device.configDone {
		panic("Cannot setFormat on property " + p.id + " for node " + p.node.id + " to device " + p.node.device.name +
			" after calling device.Run()")
	}

	validateFormat(*p, format)
	p.format = format
}

func (p *Property) SetProperty() PropertyMessage {
	var m PropertyMessage

	m.Qos = 1
	m.Retained = true
	m.property = p

	return m
}

func (p *Property) publish(topic, payload string) {
	return p.node.publish(p.id + "/" + topic, payload)
}

func (p *Property) processConnect() {
	var t string

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
	p.node.publish(p.id, p.value)
}

func validateValue(p *Property, value string) error {
	// TODO add checking
	return nil
}

// Returns an error if the property's value is wrong format, unit, or whatever.
func (m PropertyMessage) Send(value string) error {
	m.property.value = value
	err := validateValue(m.property, value)
	if m.property.node.device.configDone {
		m.property.node.device.publishChannel <- m
	}
	return err
}

func (m PropertyMessage) publish() {
}
