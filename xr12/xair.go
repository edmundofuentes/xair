package xr12

import (
	"errors"
	"fmt"
	"github.com/rakyll/portmidi"
	"log"
	"strings"
	"time"
)

type XAir struct {
	inputStream *portmidi.Stream
	outputStream *portmidi.Stream

	midiQueue chan MidiShort
	sysExQueue chan []byte
}

/*
 	We'll handle the buses as 1-indexed (1 thru 6) with the "Bus 0" being the actual Main Output LR
 */
const BUS_MAIN = 0
const CHANNEL_MAIN = 0

const MUTE = true
const UNMUTE = false


const LEVEL_1 = 1
const LEVEL_2 = 2
const LEVEL_3 = 3
const LEVEL_4 = 4
const LEVEL_5 = 5
const LEVEL_6 = 6
const LEVEL_7 = 7


func levels() map[int]string {
	l := make(map[int]string)

	l[LEVEL_1] = "-35" 	// step 225 of 1024 -> 22% -22
	l[LEVEL_2] = "-15" 	// step 448 of 1024 -> 44% +22 -19
	l[LEVEL_3] = "-5" 	// step 640 of 1024 -> 63% +19 -6
	l[LEVEL_4] = "-2.4" // step 708 of 1024 -> 69% +6 -6
	l[LEVEL_5] = "0" 	// step 770 of 1024 -> 75% +6 -6
	l[LEVEL_6] = "2.4" 	// step 830 of 1024 -> 81% +6 -6
	l[LEVEL_7] = "5" 	// step 895 of 1024 -> 87% +6

	return l
}


func Open(deviceName string) (*XAir, error) {
	input, output, err := discover(deviceName)
	if err != nil {
		return nil, err
	}

	inStream, err := portmidi.NewInputStream(input, 1024)
	if err != nil {
		return nil, err
	}

	outStream, err := portmidi.NewOutputStream(output, 1024, 0)
	if err != nil {
		inStream.Close()
		return nil, err
	}

	return &XAir{inputStream: inStream, outputStream: outStream}, nil
}

func discover(deviceName string) (input portmidi.DeviceID, output portmidi.DeviceID, err error) {
	in := -1
	out := -1
	for i := 0; i < portmidi.CountDevices(); i++ {
		info := portmidi.Info(portmidi.DeviceID(i))

		fmt.Printf("%+v\n", info)

		if strings.Contains(info.Name, deviceName) {
			if info.IsInputAvailable {
				in = i
			}
			if info.IsOutputAvailable {
				out = i
			}
		}
	}

	if in == -1 || out == -1 {
		err = errors.New("xair: no device is connected")
	} else {
		input = portmidi.DeviceID(in)
		output = portmidi.DeviceID(out)
	}
	return
}

func (x *XAir) Run() {
	go x.runOutbound()
	go x.runInbound()
}

func (x *XAir) Close() {
	x.inputStream.Close()
	x.outputStream.Close()
}

func (x *XAir) runOutbound() {
	for {
		select {
		case out := <- x.midiQueue:
			// send the outbound midi event
			fmt.Printf("->xair [midi] %b %d %d\n", out.status, out.data1, out.data2)

			err := x.outputStream.WriteShort(out.status, out.data1, out.data2)

			if err != nil {
				log.Fatal(err)
			}

		case out := <-x.sysExQueue:
			// send the outbound sysex event
			//fmt.Printf("->xair [sysex] %s\n", fmt.Sprintf("% X", out)) // hex format

			fmt.Printf("->xair [sysex] %s\n", string(out[:]))

			payload := appendOscPrefixAndSuffic(out)

			err := x.outputStream.WriteSysExBytes(portmidi.Time(), payload)

			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (x *XAir) runInbound() {
	var inBytes []byte
	var err error

	for {
		// sleep for a while before the new polling tick,
		// otherwise operation is too intensive and blocking
		time.Sleep(50 * time.Millisecond)

		if inBytes, err = x.inputStream.ReadSysExBytes(1024); err != nil {
			break
		}

		if len(inBytes) > 0 {
			fmt.Print("<-xair ")
			printHex(inBytes)
		}
	}

	if err != nil {
		log.Fatal(err)
	}
}

func (x *XAir) queueMidiMessage(m MidiShort) {
	select {
	case x.midiQueue <- m:
	default:
		log.Fatal("xair: midi outbound queue is full")
	}
}


func (x *XAir) queueSysExMessage(m []byte) {
	select {
	case x.sysExQueue <- m:
	default:
		log.Fatal("xair: sysex outbound queue is full")
	}
}


// Actions
func (x *XAir) ChannelMute(channel int, mute bool) {
	var value int64

	if mute {
		value = 127
	} else {
		value = 0
	}

	var midi MidiShort
	if channel == CHANNEL_MAIN {
		// The actual "note" value for the Main Output LR is 31
		buildMidiShort(MIDI_CONTROL_CHANGE, XAIR_MIDI_MUTE_CHANNEL, 31, value)
	} else {
		// For every other channel, it is 1-indexed in the UI, but 0-indexed via MIDI
		buildMidiShort(MIDI_CONTROL_CHANGE, XAIR_MIDI_MUTE_CHANNEL, int64(channel-1), value)
	}


	x.queueMidiMessage(midi)
}

func (x *XAir) ChannelLevel(channel, bus, level int) {
	if bus < 0 || bus > 16 {
		log.Fatalf("xair: bus out of range, got '%d', range: 0(main) and 1-16", bus)
	}

	if channel < 1 || channel > 32 {
		log.Fatalf("xair: channel out of range, got '%d', range: 1-32", channel)
	}

	for i, value := range levels() {
		if level == i {
			if bus == BUS_MAIN {
				// channels are required 1-indexed, the UI is 1-indexed, no changed need.
				x.queueSysExMessage([]byte(fmt.Sprintf("/ch/%02d/mix/fader %s", channel, value)))
				return
			} else {
				// channels and buses are required 1-indexed, the UI is 1-indexed, no changed need.
				x.queueSysExMessage([]byte(fmt.Sprintf("/ch/%02d/mix/%02d/level %s", channel, bus, value)))
				return
			}
		}
	}

	log.Fatalf("xair: unknown level step '%d', available: 1-7", level)
}

func (x *XAir) MainLevel(level int) {
	for i, value := range levels() {
		if level == i {
			x.queueSysExMessage([]byte(fmt.Sprintf("/main/st/mix/fader %s", value)))
			return
		}
	}

	log.Fatalf("xair: unknown level step '%d', available: 1-7", level)
}

func (x *XAir) BusLevel(bus, level int) {
	if bus < 1 || bus > 16 {
		log.Fatalf("xair: bus out of range, got '%d', range: 1-16", bus)
	}

	for i, value := range levels() {
		if level == i {
			// buses are required 1-indexed, the UI is 1-indexed, no changed need.
			x.queueSysExMessage([]byte(fmt.Sprintf("/bus/%02d/mix/fader %s", bus, value)))
			return
		}
	}

	log.Fatalf("xair: unknown level step '%d', available: 1-7", level)
}


func (x *XAir) TriggerMidiDump() {
	// Special command "MIDI Dump" triggered by sending the bytes: B0 7F 7F
	midi := buildMidiShort(MIDI_CONTROL_CHANGE, 1, 127, 127)

	x.queueMidiMessage(midi)
}