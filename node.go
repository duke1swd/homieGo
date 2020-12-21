package homie

// Node methods

// Create and return a node
func (device Device) NewNode(id, name, nType string) Node {
	var node Node

	id = validate(id, false)

	if device.configDone {
		panic("Cannot add node " + id + " to device " + device.name +
			" after calling device.Run()")
	}

	if _, ok := device.nodes[id]; ok {
		panic("Device " + device.name + " already has a node " + id)
	}

	node.name = name
	node.nType = nType
	node.properties = make(map[string]Property)

	device.nodes[id] = node

	return node
}

func (n Node) Id() string {
	return n.id
}

func (n Node) Name() string {
	return n.name
}

func (n Node) NodeType() {
	return nType
}
