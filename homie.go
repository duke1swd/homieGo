package homie

import (
//"github.com/eclipse/paho.mqtt.golang"
)

// Type hierarchy

// All levels can have attributes, but at Device and Node level they are controller
// by this code.
type Attribute struct {
	id    string
	value string
}

type Property struct {
	id         string
	settable   bool // hardwired attribute
	attributes map[string]Attribute
}

type Node struct {
	id         string
	nType      string
	properties map[string]Property
}

type Device struct {
	id             string
	protocol       string          // Homie level.  Always 4.0.1
	name           string          // Friendly name
	state          string          // Fixed set of states possible
	nodes          map[string]Node // indexed by node ID
	extensions     string          // always null string
	implementation string          // always "homieGo"
	configDone     bool            // 2 states, configuring and configured
}

var (
	devices map[string]Device
)

func init() {
	devices = make(map[string]Device)
}
