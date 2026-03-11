package main

import (
	"encoding/binary"

	webrtcvad "github.com/maxhawkins/go-webrtcvad"
)

const vadFrameMs = 20 // 20ms frames at 16kHz = 320 samples

var vadFrameSize = asrSampleRate * vadFrameMs / 1000 // 320

// hasVoice returns true if any 20ms frame in samples contains speech
// according to WebRTC VAD (mode 3 — most aggressive filtering).
func hasVoice(samples []int16) bool {
	if len(samples) < vadFrameSize {
		return false
	}

	vad, err := webrtcvad.New()
	if err != nil {
		return true // fail open
	}
	if err := vad.SetMode(3); err != nil {
		return true
	}

	for off := 0; off+vadFrameSize <= len(samples); off += vadFrameSize {
		frame := samplesToBytes(samples[off : off+vadFrameSize])
		active, err := vad.Process(asrSampleRate, frame)
		if err != nil {
			return true
		}
		if active {
			return true
		}
	}
	return false
}

// tailHasVoice checks only the last n samples for voice activity.
func tailHasVoice(samples []int16, n int) bool {
	start := len(samples) - n
	if start < 0 {
		start = 0
	}
	return hasVoice(samples[start:])
}

// samplesToBytes converts int16 PCM to little-endian bytes for WebRTC VAD.
func samplesToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}
