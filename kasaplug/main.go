package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/duke1swd/homieGo/library"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	broadcastAddr   *net.UDPAddr
	broadcastPeriod time.Duration
	debugRunLength  time.Duration
	topicBase       string
	statusQueryB    []byte
	setOnB          []byte
	setOffB         []byte
	packetConn      net.PacketConn
)

type kasaDevice struct {
	uid  string
	id   string // homie id
	name string // homie friendly name
	addr net.Addr
	on   bool

	lastSeen       time.Time
	hDevice        *homie.Device
	outletProperty *homie.Property
	cancelFunction context.CancelFunc
	waitChan       chan bool
}

const defaultNetwork = "192.168.1.0/24"
const defaultBroadcastPeriod = "10" // in seconds
const defaultDebugRunLength = "10"  // in seconds
const defaultLogDirectory = "/var/log"
const logFileName = "HomeAutomationLog"
const defaultTopicBase = "devices"
const kasaPort = 9999
const statusQuery = "{\"system\":{\"get_sysinfo\":null},\"emeter\":{\"get_realtime\":null}}"
const setOn = "{\"system\":{\"set_relay_state\":{\"state\":1}}}"
const setOff = "{\"system\":{\"set_relay_state\":{\"state\":0}}}"

var (
	debug             bool
	debugV            bool
	network           string
	lostDeviceTimeout time.Duration
	mqttBroker        string
	fullLogFileName   string
)

var (
	kasaMap map[string]*kasaDevice // only accessed in the context of the run() go routine.
)

func init() {
	var logDirectory string

	flag.BoolVar(&debug, "d", false, "debugging")
	flag.BoolVar(&debugV, "D", false, "extreme debugging")

	flag.Parse()

	if debugV {
		debug = true
	}

	kasaMap = make(map[string]*kasaDevice)

	if n, ok := os.LookupEnv("NETWORK"); ok {
		network = n
	} else {
		network = defaultNetwork
	}

	s := defaultBroadcastPeriod
	if debug {
		s = "1" // one second
	}
	if d, ok := os.LookupEnv("BROADCASTPERIOD"); ok {
		s = d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		n, _ = strconv.Atoi(defaultBroadcastPeriod)
	}
	broadcastPeriod = time.Duration(n) * time.Second

	s = defaultDebugRunLength
	if d, ok := os.LookupEnv("DEBUGRUNLENGTH"); ok {
		s = d
	}
	n, err = strconv.Atoi(s)
	if err != nil {
		n, _ = strconv.Atoi(defaultDebugRunLength)
	}
	debugRunLength = time.Duration(n) * time.Second

	if s, ok := os.LookupEnv("MQTTBROKER"); ok {
		mqttBroker = s
	}

	if l, ok := os.LookupEnv("LOGDIR"); ok {
		logDirectory = l
	} else {
		logDirectory = defaultLogDirectory
	}
	fullLogFileName = filepath.Join(logDirectory, logFileName)

	if debug {
		topicBase = "kasadebug"
	} else if t, ok := os.LookupEnv("HOMIETOPIC"); ok {
		topicBase = t
	} else {
		topicBase = defaultTopicBase
	}

	statusQueryB = []byte(statusQuery)
	tpEncode(statusQueryB)
	setOnB = []byte(setOn)
	tpEncode(setOnB)
	setOffB = []byte(setOff)
	tpEncode(setOffB)

	lostDeviceTimeout = broadcastPeriod * time.Duration(10)
}

func tpEncode(data []byte) {
	var k byte

	k = 171
	for i := 0; i < len(data); i++ {
		data[i] = data[i] ^ k
		k = data[i]
	}
}

func tpDecode(data []byte) {
	var k byte

	k = 171
	for i := 0; i < len(data); i++ {
		t := data[i] ^ k
		k = data[i]
		data[i] = t
	}
}

func printData(data []byte) {
	if !debugV {
		return
	}

	for _, v := range data {
		fmt.Printf("%2x ", v)
	}
	fmt.Printf("\n")
}

// Broadcast a packet.  Packet must already be encrypted
func broadcast(data []byte) {

	if debugV {
		fmt.Println("Broadcast")
		fmt.Printf("\tAddr = %v, port = %v\n", broadcastAddr.IP, broadcastAddr.Port)
	}

	_, err := packetConn.WriteTo(data, broadcastAddr)
	if err != nil {
		logMessage(fmt.Sprintf("Kasaplug: Broadcast to %v failed with error %v", broadcastAddr, err))
	}
}

// Send a packet via UDP.  Packet must already be encrypted
func unicast(data []byte, kasa *kasaDevice) {

	_, err := packetConn.WriteTo(data, kasa.addr)
	if err != nil {
		logMessage(fmt.Sprintf("Kasaplug: Unicast to %v failed with error %v", kasa.addr, err))
	}
}

// Runs as an indepent routine.  Listens for responses to our broadcast
// parses them, mostly, and sends them down the channel
func listenerSysinfo(c context.Context, output chan map[string]interface{}) {
	var response interface{}

	// if the context dies, kill any pending read
	go func() {
		for _ = range c.Done() {
		}
		packetConn.SetReadDeadline(time.Now())
	}()

	largeBuf := make([]byte, 1024)
	for {
		if c.Err() != nil {
			// context timed out or was cancelled
			return
		}

		n, addr, err := packetConn.ReadFrom(largeBuf)
		if err != nil {
			if c.Err() != nil {
				// context timed out or was cancelled
				return
			}
			continue
		}
		buf := largeBuf[:n] // throw away the rest of the buffer
		tpDecode(buf)
		if debugV {
			fmt.Printf("Got %d bytes from addr %v: %s\n", n, addr, string(buf))
		} else if debug {
			fmt.Printf("Got %d bytes from addr %v\n", n, addr)
		}

		// This long string of code peels away fluff and leaves us with the sysinfo object.
		response = nil
		if err := json.Unmarshal(buf, &response); err != nil {
			// not valid json
			if debug {
				fmt.Printf("Not valid json: %v\n", err)
			}
			continue
		}
		rmap, ok := response.(map[string]interface{})
		if !ok {
			if debug {
				fmt.Printf("response is not a map\n")
			}
			continue
		}
		if debugV {
			fmt.Printf("response keys found:\n")
			for k, _ := range rmap {
				fmt.Printf("\t%s\n", k)
			}
		}
		s, ok := rmap["system"]
		if !ok {
			if debug {
				fmt.Printf("No system entry in response json\n")
			}
			continue
		}

		smap, ok := s.(map[string]interface{})
		if !ok {
			if debug {
				fmt.Printf("System entry is not a map\n")
			}
			continue
		}

		if debugV {
			fmt.Printf("system keys found:\n")
			for k, _ := range smap {
				fmt.Printf("\t%s\n", k)
			}
		}

		g, ok := smap["get_sysinfo"]
		if !ok {
			if debug {
				fmt.Printf("No get_sysinfo entry in response json\n")
			}
			continue
		}

		gmap, ok := g.(map[string]interface{})
		if !ok {
			if debug {
				fmt.Printf("Get_sysinfo entry is not a map\n")
			}
			continue
		}

		if debugV {
			fmt.Printf("get_sysinfo keys found:\n")
			for k, _ := range gmap {
				fmt.Printf("\t%s\n", k)
			}
		}
		gmap["addr"] = addr
		output <- gmap
	}
}

// converts a plug's nickname into a valid homie id and name.
func homieName(alias string) (string, string) {
	id := make([]byte, 0, len(alias))

	for i, b := range []byte(alias) {
		switch {
		case (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z'):
			id = append(id, b)
		case i > 0 && (b == '-' || b == ' ' || b == '_'):
			id = append(id, '-')
		}
	}
	return strings.ToLower(string(id)), alias
}

// Convert the json stuff that came back from the tp-link to our kasaDevice
func tp2kasa(gmap map[string]interface{}) (*kasaDevice, bool) {

	var kasa kasaDevice

	a, ok := gmap["alias"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no alias\n")
		}
		return nil, false
	}
	alias, ok := a.(string)
	if !ok {
		if debug {
			fmt.Printf("gmap alias is not a string (!)\n")
		}
		return nil, false
	}
	kasa.id, kasa.name = homieName(alias)

	ad, ok := gmap["addr"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no addr\n")
		}
		return nil, false
	}
	kasa.addr, ok = ad.(net.Addr)
	if !ok {
		if debug {
			fmt.Printf("gmap addr is not of type net.Addr\n")
		}
		return nil, false
	}

	d, ok := gmap["deviceId"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no deviceId\n")
		}
		return nil, false
	}
	kasa.uid, ok = d.(string)
	if !ok {
		if debug {
			fmt.Printf("gmap deviceId is not a string (!)\n")
		}
		return nil, false
	}

	r, ok := gmap["relay_state"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no relay_state\n")
		}
		return nil, false
	}
	relay, ok := r.(float64)
	if !ok {
		if debug {
			fmt.Printf("gmap relay_state (%v) is not an float (!)\n", relay)
		}
		return nil, false
	}
	switch int(relay) {
	case 0:
		kasa.on = false
	case 1:
		kasa.on = true
	default:
		if debug {
			fmt.Printf("gmap relay_state(%f) is not 0 or 1\n", relay)
		}
		return nil, false
	}

	return &kasa, true
}

// This function processes a request to turn the outlet on or off
var namesForOnOff map[string]bool = map[string]bool{
	"on":    true,
	"ON":    true,
	"1":     true,
	"true":  true,
	"off":   false,
	"OFF":   false,
	"0":     false,
	"false": false,
}

func (kasa *kasaDevice) setValue(value string) {
	// If we don't understand the command, do nothing
	newVal, ok := namesForOnOff[value]
	if !ok {
		logMessage(fmt.Sprintf("Kasaplug: Unknown command %s", value))
		return
	}

	if kasa.on == newVal {
		return
	}

	setRelayState(kasa, newVal)
	unicast(statusQueryB, kasa) // ping the device.  the response to this will trigger the setting of the property
}

// This function creates the homie device, its node, and its properties
func createHomieDevice(kasa *kasaDevice) {
	var c context.Context

	logMessage(fmt.Sprintf("Kasaplug: Creating device for %s with ID %s", kasa.name, kasa.id))

	// create the device
	kasa.hDevice = homie.NewDevice(kasa.id, kasa.name)
	if len(topicBase) > 0 {
		kasa.hDevice.SetTopicBase(topicBase)
	}
	if len(mqttBroker) > 0 {
		kasa.hDevice.SetMqttBroker(mqttBroker)
	}

	// it has one node
	node := kasa.hDevice.NewNode("outlet", "outlet", "relay", nil)
	property := node.Advertise("on", "on", homie.DtString)
	property.Settable(func(d *homie.Device, n *homie.Node, p *homie.Property, value string) bool {
		kasa.setValue(value)
		return true
	})

	// the node has one property
	kasa.outletProperty = property
	c, kasa.cancelFunction = context.WithCancel(context.Background())
	kasa.waitChan = make(chan bool, 1)

	// start the device go routine
	go kasa.hDevice.RunWithContext(c, kasa.waitChan)

	// publish the device's status
	s := "false"
	if kasa.on {
		s = "true"
	}
	property.SetProperty().Send(s)
}

func destroyHomieDevice(kasa *kasaDevice) {

	logMessage(fmt.Sprintf("Kasaplug: Destroying kasa device %s", kasa.name))
	delete(kasaMap, kasa.uid)
	kasa.cancelFunction()
	for _ = range kasa.waitChan {
	}
	kasa.hDevice.Destroy()
}

// Process events and do things
func run(c context.Context, deviceChannel chan map[string]interface{}) {
	ticker := time.NewTicker(lostDeviceTimeout / time.Duration(2))
	for {
		select {
		case gmap := <-deviceChannel:
			kasa, ok := tp2kasa(gmap)
			if !ok {
				break
			}

			if debug {
				fmt.Printf("Got device %s\n", kasa.name)
				fmt.Printf("\tRelay is On: %v\n", kasa.on)
				fmt.Printf("\tAddress: %s\n", kasa.addr.String())
			}

			if oldK, ok := kasaMap[kasa.uid]; ok {

				// If device address changes, record change.  No other work
				oldK.addr = kasa.addr

				// device already exists.
				// uid already matches
				if oldK.id != kasa.id || oldK.name != kasa.name {
					// device has been programmed to a new identity
					// destroy the old device and create the new one
					destroyHomieDevice(oldK)
					createHomieDevice(kasa)
					oldK = kasa
				}

				// Process change of device state
				// record the new state and tell Homie about it.
				if oldK.on != kasa.on {
					oldK.on = kasa.on
					s := "false"
					if kasa.on {
						s = "true"
					}
					oldK.outletProperty.SetProperty().Send(s)
				}

				// Mark alive
				oldK.lastSeen = time.Now()
				kasaMap[kasa.uid] = oldK
			} else {
				// New device.  Create it
				createHomieDevice(kasa)
				kasa.lastSeen = time.Now()
				kasaMap[kasa.uid] = kasa
			}

			//_, ok := callTCP(kasa, "{\"system\":{\"set_relay_state\":{\"state\":1}}}")
			//fmt.Printf("call returns %v\n", ok)

		case _ = <-ticker.C:
			// Scan for any devices we have not seen in awhile.
			for _, k := range kasaMap {
				if time.Since(k.lastSeen) > lostDeviceTimeout {
					destroyHomieDevice(k)
				}
			}

		case <-c.Done():
			return
		}
	}
}

// set the relay state to 0 (off) or 1 (on)
func setRelayState(k *kasaDevice, state bool) {
	var command []byte

	if state {
		command = setOnB
	} else {
		command = setOffB
	}

	if _, err := packetConn.WriteTo(command, k.addr); err != nil {
		logMessage(fmt.Sprintf("Kasaplug: Send command to %s (%v) failed with error %v", k.name, k.addr, err))
	}
}

// Call and response to the device over TCP
// Function works, but is currently unused.
func callTCP(device kasaDevice, call string) (interface{}, bool) {
	var dialer net.Dialer
	dialer.Timeout = time.Duration(2) * time.Second // the device responds quickly or not at all

	conn, err := dialer.Dial("tcp", device.addr.String())
	if err != nil {
		logMessage(fmt.Sprintf("Kasaplug: dial out to device via tcp failed: %v", err))
		return nil, false
	}
	defer conn.Close()

	// First, tell the device how long the message will be
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(call)))
	n, err := conn.Write(lengthBytes)
	if n != 4 {
		if debug {
			fmt.Printf("Wrote %d bytes rather than 4\n", n)
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on writing length to TCP: %v\n", err)
		}
		return nil, false
	}

	// Now, write the message
	writeBuffer := []byte(call)
	tpEncode(writeBuffer)
	n, err = conn.Write(writeBuffer)
	if n != len(call) {
		if debug {
			fmt.Printf("Wrote %d bytes rather than %d\n", n, len(call))
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on writing message to TCP: %v\n", err)
		}
		return nil, false
	}

	// Now, read back the length
	n, err = conn.Read(lengthBytes)
	if n != 4 {
		if debug {
			fmt.Printf("Read %d bytes rather than 4\n", n)
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on reading length from TCP: %v\n", err)
		}
		return nil, false
	}
	lenToRead := int(binary.BigEndian.Uint32(lengthBytes))
	readBuffer := make([]byte, lenToRead)

	// Read the response message
	n, err = conn.Read(readBuffer)
	if n != lenToRead {
		if debug {
			fmt.Printf("Read %d bytes rather than %d\n", n, lenToRead)
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on reading message from TCP: %v\n", err)
		}
		return nil, false
	}
	tpDecode(readBuffer)
	if debugV {
		fmt.Printf("Response is %s\n", string(readBuffer))
	}
	return nil, true
}

func lastAddr(n *net.IPNet) (net.IP, error) { // works when the n is a prefix, otherwise...
	if n.IP.To4() == nil {
		return net.IP{}, errors.New("does not support IPv6 addresses.")
	}
	ip := make(net.IP, len(n.IP.To4()))
	binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(n.IP.To4())|^binary.BigEndian.Uint32(net.IP(n.Mask).To4()))
	return ip, nil
}

func setupBroadcastAddr(port int) {
	var u net.UDPAddr

	_, n, err := net.ParseCIDR(network)
	if err != nil {
		panic(err)
	}

	b, err := lastAddr(n)
	if err != nil {
		panic(err)
	}

	u.IP = b
	u.Port = port
	broadcastAddr = &u
}

func broadcaster(c context.Context) {
	ticker := time.NewTicker(broadcastPeriod)

	for {
		select {
		case <-c.Done():
			return
		case _ = <-ticker.C:
			broadcast(statusQueryB)
		}
	}
}

func nilCancel() {
}

func main() {
	setupBroadcastAddr(kasaPort)
	var (
		err error
		c   context.Context
		cfl context.CancelFunc
	)

	// Create a packet connection
	packetConn, err = net.ListenPacket("udp", "") // listen for UDP on unspecified port
	if err != nil {
		panic(err)
	}
	defer packetConn.Close()

	deviceChannel := make(chan map[string]interface{}, 100)

	if debug {
		c, cfl = context.WithTimeout(context.Background(), debugRunLength)
		defer cfl()
	} else {
		c = context.Background()
		cfl = nilCancel
	}

	go listenerSysinfo(c, deviceChannel)

	// Set up periodic broadcasts
	go broadcaster(c)

	// Collect the resposes

	if debugV {
		fmt.Printf("running\n")
	}
	logMessage("Kasaplug: Running")

	run(c, deviceChannel)

	if debugV {
		fmt.Printf("run returned\n")
	}
}

func logMessage(m string) {
	formattedMsg := time.Now().Format("Mon Jan 2 15:04:05 2006") + "  " + m + "\n"

	// when containerized, log messages should go to stdout
	if fullLogFileName[0] == '-' {
		fmt.Print(formattedMsg)
	} else {
	// when running as a daemon, log files should go in the /var/log directory
		logMessageFile(formattedMsg)
	}
}

func logMessageFile(formattedMsg string) {
	f, err := os.OpenFile(fullLogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Logger: Cannot open for writing log file %s. err = %v", fullLogFileName, err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(formattedMsg)
	if err != nil {
		log.Printf("Logger: Error writing to file %s.  err = %v\n", fullLogFileName, err)
		return
	}
	if debug {
		fmt.Print(formattedMsg)
	}
}
