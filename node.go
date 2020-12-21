package homie

// Node methods

// Create and return a node
func NewNode(id, nType string) Node {
	var node Node

	node.id = validate(id)
	node.nType = nType
	node.properties = make(map[string]Property)

	return node
}

func (n Node) Id() string {
	return n.id
}
