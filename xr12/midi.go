package xr12

const MIDI_NOTE_OFF 		int64 = 0b1000
const MIDI_NOTE_ON 			int64 = 0b1001
const MIDI_CONTROL_CHANGE 	int64 = 0b1011
const MIDI_PROGRAM_CHANGE 	int64 = 0b1100

const XAIR_MIDI_MUTE_CHANNEL = 2

type MidiShort struct {
	status int64
	data1 int64
	data2 int64
}

func buildMidiShort(cmd, channel, note, value int64) MidiShort {
	return MidiShort {
		status: (cmd << 4 & 0xF0) | ((channel-1) & 0xF), // channels are zero indexed!
		data1: note,
		data2: value,
	}
}