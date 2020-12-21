An implementation of the Homie IOT protocol in Go.

Under development, not yet doing anything useful.


NOTES

Errors
	Functions return errors when there is some reasonable hope
	that the caller might do something with the error.  Things like
	a temporary lack of connection to the MQTT broker cause errors.

	In an effort to reduce error returns and the checking they
	require, functions panic when they detect errors that are
	unlikely to be corrected.  For example, providing a name for
	an object that does not conform to Homie's restrictions on
	names (IDs) will produce a panic.

Flow of Control
	One configures a homie device, then calls device.Run().
	device.Run() never returns.  However, it will make callbacks
	to supplied setup, loop, and loopConnected functions.

	If one is managing multiple devices, then each device needs
	a separate go routine to call device.Run().

	Event handlers are called out of mqtt message handlers,
	and run asynchronously to the device.Run() thread.

	Event handlers must not block.  Note that calling mqtt
	may result in blocking and other bad behavior.  Do not
	call mqtt directly from the event handlers.

	Homie calls to manage properties are safe to call from
	within event handlers.

Timing of run loop
	There are 3 ways handle the run loop timing.  Normally
	device.Run() wakes up every 0.25 seconds and calls the loop
	callback functions.  This period is configurable.  The loop
	callback functions should be non-blocking.

	If the device.Run() period is set to zero and the loop
	callback functions are non-blocking, then the device.Run()
	loop will consume an entire thread.  This is appropriate for
	small systems that need to poll external hardware as often
	as possible.

	One can set the device.Run() period to zero and block for
	small amounts of time in the loop callback.  One should never
	block for two long, as device.Run() needs the CPU to process
	mqtt messages from time to time.

Types
	devices have nodes
	nodes have properties
	properties have attributes

Range Nodes
	The convention doesn't speak to range nodes.  However, ESP8266
	implementation has them.  Basically a range node is a short-hand
	way of dealing with a device with a bunch of similar nodes.
	This code will handle them, but doesn't yet.

	Note: When we do handle them, we'll call them "spans" rather than
	"ranges".  This avoids the mess that comes from "range" being a 
	Go keyword.
