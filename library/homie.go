package homie

import (
	"github.com/eclipse/paho.mqtt.golang"
	"time"
)

const mqttClientIDPrefix = "homieGo"
const defaultTopicBase = "homie"
const defaultMqttBroker = "tcp://127.0.0.1:1883"

// Type hierarchy

// These are the allow Property data types, as per v4.0.0 convention
const (
	DtString = iota
	DtInteger
	DtFloat
	DtBoolean
	DtEnum
	DtColor
)

// These are the allowed Property units.  Units however, are optional.
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
	id       string
	name     string
	node     *Node
	settable bool // hardwired attribute
	dataType int  // must be one of the defined data types
	handler  func(d *Device, n *Node, p *Property, value string) bool
	format   string
	unit     string
	value    string
}

type PropertyMessage struct {
	property *Property
	Qos      byte // default value is 1
	Retained bool // default value is true
}

type Node struct {
	id         string
	device     *Device
	name       string
	nType      string
	handler    func(d *Device, n *Node, p *Property, value string) bool
	properties map[string]*Property
}

type Device struct {
	id               string
	protocol         string           // Homie level.  Always 4.0.1
	name             string           // Friendly name
	state            string           // Fixed set of states possible
	nodes            map[string]*Node // indexed by node ID
	extensions       string           // We currently support two, legacy-stats and legacy-firmware
	implementation   string           // always "homieGo"
	configDone       bool             // 2 states, configuring and configured
	connected        bool
	topicBase        string // default is "homie"
	period           time.Duration
	globalHandler    func(d *Device, n *Node, p *Property, value string) bool
	broadcastHandler func(d *Device, level, value string)
	loop             func(d *Device)
	mqttBroker       string
	clientOptions    *mqtt.ClientOptions
	client           mqtt.Client

	unsubscribes []func()

	// Stuff for the stats extension.  At the moment all we do is publish uptime.
	statsInterval time.Duration // how often to publish stats
	statsBootTime time.Time     // used to compute uptime

	// Stuff for the firmware extension.
	localIP   string // NYI
	mac       string // NYI
	fwName    string
	fwVersion string

	// This channel is used to ensure that messages are not sent from an event handler
	publishChannel chan PropertyMessage

	// This channel reflects connection status changes back to the run() method from the event handler.
	connectChannel chan bool

	// This channel is used to process publish tokens at the right time and place, asynchronously
	tokenChannel chan *mqtt.Token
}

var (
	devices map[string]*Device
)

func init() {
	devices = make(map[string]*Device)
}
