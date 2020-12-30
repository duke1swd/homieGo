package homie

// Node methods

// Create and return a node
func (device *Device) NewNode(id, name, nType string, handler func(d *Device, n *Node, p *Property, a string)) *Node {
	var node Node

	id = validate(id, false)

	if device.configDone {
		panic("Cannot add node " + id + " to device " + device.name +
			" after calling device.Run()")
	}

	if _, ok := device.nodes[id]; ok {
		panic("Device " + device.name + " already has a node " + id)
	}

	node.id = id
	node.name = name
	node.nType = nType
	node.properties = make(map[string]*Property)
	node.handler = handler
	node.device = device

	device.nodes[id] = &node

	return &node
}

func (n *Node) Advertise(id, name string, dataType int) *Property {
	var property Property

	id = validate(id, false)

	if n.device.configDone {
		panic("Cannot add property " + id + " to node " + n.name +
			" after calling device.Run()")
	}

	if _, ok := n.properties[id]; ok {
		panic("duplicate property id " + id + " in node " + n.name)
	}

	property.id = id
	property.name = name
	property.settable = false

	switch dataType {
	case DtString:
	case DtInteger:
	case DtFloat:
	case DtBoolean:
	case DtEnum:
	case DtColor:
	default:
		panic("Invalid data type supplied for property " + id + " in node " + n.name)
	}
	property.dataType = dataType

	property.format = ""
	property.unit = ""

	property.handler = nil
	n.properties[id] = &property
	property.node = n

	return &property
}

func (n Node) Id() string {
	return n.id
}

func (n Node) Name() string {
	return n.name
}

func (n Node) NodeType() string {
	return n.nType
}

func (n *Node) publish(topic, payload string) {
	n.device.publish(n.id+"/"+topic, payload)
}

func (n *Node) processConnect() {
	n.publish("$name", n.name)
	n.publish("$type", n.nType)

	// Spit out the properties
	if len(n.properties) > 0 {
		s := ""
		for n, _ := range n.properties {
			if len(s) > 0 {
				s = s + "," + n
			} else {
				s = n
			}
		}
		n.publish("$properties", s)

		for _, p := range n.properties {
			p.processConnect()
		}
	} else {
		n.publish("$properties", "")
	}
}
