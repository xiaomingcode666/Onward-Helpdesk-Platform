package services

import (
	"encoding/binary"
	"testing"
)

func TestDecodePCM16WAV(t *testing.T) {
	raw := makePCM16WAV(16_000, 1)
	pcm, duration, err := decodePCM16WAV(raw)
	if err != nil {
		t.Fatalf("decodePCM16WAV() error = %v", err)
	}
	if len(pcm) != 32_000 || duration != 1 {
		t.Fatalf("decoded PCM = %d bytes, duration = %d; want 32000 bytes and 1 second", len(pcm), duration)
	}
}

func TestDecodePCM16WAVRejectsNonPCM(t *testing.T) {
	raw := makePCM16WAV(8_000, 2)
	if _, _, err := decodePCM16WAV(raw); err == nil {
		t.Fatal("decodePCM16WAV accepted an unsupported sample rate")
	}
}

func makePCM16WAV(sampleRate, seconds int) []byte {
	dataSize := sampleRate * seconds * 2
	raw := make([]byte, 44+dataSize)
	copy(raw[0:4], "RIFF")
	binary.LittleEndian.PutUint32(raw[4:8], uint32(36+dataSize))
	copy(raw[8:12], "WAVE")
	copy(raw[12:16], "fmt ")
	binary.LittleEndian.PutUint32(raw[16:20], 16)
	binary.LittleEndian.PutUint16(raw[20:22], 1)
	binary.LittleEndian.PutUint16(raw[22:24], 1)
	binary.LittleEndian.PutUint32(raw[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(raw[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(raw[32:34], 2)
	binary.LittleEndian.PutUint16(raw[34:36], 16)
	copy(raw[36:40], "data")
	binary.LittleEndian.PutUint32(raw[40:44], uint32(dataSize))
	return raw
}
