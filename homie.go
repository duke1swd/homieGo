package homie

import (
	"time"
	//"github.com/eclipse/paho.mqtt.golang"
)

// Type hierarchy

// All levels can have attributes, but at Device and Node level they are controller
// by this code.
type Attribute struct {
	id    string
	value string
}

// These are the allow Property data types, as per v4.0.0 convention
const (
	DtString = iota
	DtInteger
	DtFloat
	DtPercent
	DtBoolean
	DtEnum
	DtColor
	DtDateTime
	DtDuration
)

// These are the allowed units.  Units however, are optional.
var propertyUnits map[string]bool = map[string]bool{
	"°C":     true, // degrees C
	"°F":     true, // degrees F
	"°":      true, // degrees (angle)
	"L":      true, // liters
	"gal":    true, // gallons
	"V":      true, // volts
	"W":      true, // watts
	"A":      true, // amps
	"%":      true, // percentage
	"m":      true, // meters
	"ft":     true, // feet
	"pascal": true, // Pascal
	"psi":    true, // PSI
	"#":      true, // count or amount
}

type Property struct {
	id         string
	node       *Node
	settable   bool // hardwired attribute
	dataType   int  // must be one of the defined data types
	handler    func(d Device, n Node, p Property, a string)
	format     string
	unit       string
	attributes map[string]Attribute
}

type PropertyMessage struct {
	property *Property
	Qos      int  // default value is 1
	Retained bool // default value is true
}

type Node struct {
	id         string
	device     *Device
	name       string
	nType      string
	handler    func(d Device, n Node, p Property, a string)
	properties map[string]Property
}

type Device struct {
	id               string
	protocol         string          // Homie level.  Always 4.0.1
	name             string          // Friendly name
	state            string          // Fixed set of states possible
	nodes            map[string]Node // indexed by node ID
	extensions       string          // We currently support two, legacy-stats and legacy-firmware
	implementation   string          // always "homieGo"
	configDone       bool            // 2 states, configuring and configured
	connected        bool
	globalHandler    func(d Device, n Node, p Property, a Attribute, value string)
	broadcastHandler func(d Device, level, value string)
	loop             func(d Device)

	// Stuff for the stats extension.  At the moment all we do is publish uptime.
	statsInterval time.Duration // how often to publish stats
	statsBootTime time.Time     // used to compute uptime

	// Stuff for the firmware extension.
	localIP   string // NYI
	mac       string // NYI
	fwName    string
	fwVersion string
}

var (
	devices map[string]Device
)

func init() {
	devices = make(map[string]Device)
}
