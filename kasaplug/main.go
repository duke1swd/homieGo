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
	"strconv"
	"time"
)

var (
	broadcastAddr   *net.UDPAddr
	broadcastPeriod time.Duration
	plugs           map[string]homie.Device = make(map[string]homie.Device)
	statusQueryB    []byte
	setOnB          []byte
	setOffB         []byte
)

type kasaDevice struct {
	uid      string
	id       string // homie id
	name     string // homie friendly name
	addr     net.Addr
	on       bool
	hDevice  *homie.Device
	lastSeen time.Time
}

const defaultNetwork = "192.168.1.0/24"
const defaultBroadcastPeriod = "10" // in seconds
const kasaPort = 9999
const statusQuery = "{\"system\":{\"get_sysinfo\":null},\"emeter\":{\"get_realtime\":null}}"
const setOn = "{\"system\":{\"set_relay_state\":{\"state\":1}}}"
const setOff = "{\"system\":{\"set_relay_state\":{\"state\":0}}}"

var (
	debug   bool
	debugV  bool
	network string
)

var (
	kasaMap map[string]*kasaDevice // only accessed in the context of the run() go routine.
)

func init() {
	flag.BoolVar(&debug, "d", false, "debugging")
	flag.BoolVar(&debugV, "D", false, "extreme debugging")

	flag.Parse()

	kasaMap = make(map[string]*kasaDevice)

	if n, ok := os.LookupEnv("NETWORK"); ok {
		network = n
	} else {
		network = defaultNetwork
	}

	s := defaultBroadcastPeriod
	if d, ok := os.LookupEnv("BROADCASTPERIOD"); ok {
		s = d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		n, _ = strconv.Atoi(defaultBroadcastPeriod)
	}
	broadcastPeriod = time.Duration(n) * time.Second

	statusQueryB = []byte(statusQuery)
	tpEncode(statusQueryB)
	setOnB = []byte(setOn)
	tpEncode(setOnB)
	setOffB = []byte(setOff)
	tpEncode(setOffB)
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
func broadcast(pc net.PacketConn, data []byte) {

	_, err := pc.WriteTo(data, broadcastAddr)
	if err != nil {
		log.Printf("Broadcast to %v failed with error %v", broadcastAddr, err)
	}
}

// Runs as an indepent routine.  Listens for responses to our broadcast
// parses them, mostly, and sends them down the channel
func listenerSysinfo(c context.Context, pc net.PacketConn, output chan map[string]interface{}) {
	var response interface{}

	// if the context dies, kill any pending read
	go func() {
		for _ = range c.Done() {
		}
		pc.SetReadDeadline(time.Now())
	}()

	largeBuf := make([]byte, 1024)
	for {
		if c.Err() != nil {
			// context timed out or was cancelled
			return
		}

		n, addr, err := pc.ReadFrom(largeBuf)
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
	return alias, alias
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
	name, ok := a.(string)
	if !ok {
		if debug {
			fmt.Printf("gmap alias is not a string (!)\n")
		}
		return nil, false
	}
	kasa.id, kasa.name = homieName(name)

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

// Process events and do things
func run(c context.Context, deviceChannel chan map[string]interface{}) {
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
				// device already exists.
				// uid already matches
				if oldK.id != kasa.id || oldK.name != kasa.name {
					// device has been programmed to a new identity
					// destroy the old device and create the new one
					// TODO
				}

				// If device address changes, record change.  No other work
				oldK.addr = kasa.addr

				// Process change of device state
				// TODO
				oldK.on = kasa.on

				// Mark alive
				oldK.lastSeen = time.Now()
			} else {
				// New device.  Create it
				// TODO
				kasa.lastSeen = time.Now()
				kasaMap[kasa.uid] = kasa
			}

			//_, ok := callTCP(kasa, "{\"system\":{\"set_relay_state\":{\"state\":1}}}")
			//fmt.Printf("call returns %v\n", ok)

		case <-c.Done():
			return
		}
	}
}

// Call and response to the device over TCP
// Function works, but is currently unused.
func callTCP(device kasaDevice, call string) (interface{}, bool) {
	var dialer net.Dialer
	dialer.Timeout = time.Duration(2) * time.Second // the device responds quickly or not at all

	conn, err := dialer.Dial("tcp", device.addr.String())
	if err != nil {
		if debug {
			fmt.Printf("dial out to device via tcp failed: %v\n", err)
		}
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

func broadcaster(c context.Context, pc net.PacketConn) {
	ticker := time.NewTicker(broadcastPeriod)

	for {
		select {
		case <-c.Done():
			return
		case _ = <-ticker.C:
			broadcast(pc, statusQueryB)
		}
	}
}

func main() {
	setupBroadcastAddr(kasaPort)

	// Create a packet connection
	pc, err := net.ListenPacket("udp", "") // listen for UDP on unspecified port
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	deviceChannel := make(chan map[string]interface{}, 100)

	c, cfl := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cfl()

	go listenerSysinfo(c, pc, deviceChannel)

	// Set up periodic broadcasts
	go broadcaster(c, pc)

	// Collect the resposes

	run(c, deviceChannel)
}
