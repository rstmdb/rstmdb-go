package rstmdb

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestEncodeDecodeFrameRoundtrip(t *testing.T) {
	payload := []byte(`{"type":"request","id":"1","op":"PING"}`)
	frame, err := encodeFrame(payload)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}

	decoded, err := decodeFrame(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}

	if !bytes.Equal(payload, decoded) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decoded, payload)
	}
}

func TestEncodeFrameHeader(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	frame, err := encodeFrame(payload)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}

	// Check magic
	if string(frame[0:4]) != "RCPX" {
		t.Fatalf("magic: got %q, want RCPX", string(frame[0:4]))
	}

	// Check version
	version := binary.BigEndian.Uint16(frame[4:6])
	if version != 1 {
		t.Fatalf("version: got %d, want 1", version)
	}

	// Check flags (CRC_PRESENT)
	flags := binary.BigEndian.Uint16(frame[6:8])
	if flags != flagCRCPresent {
		t.Fatalf("flags: got 0x%04x, want 0x%04x", flags, flagCRCPresent)
	}

	// Check header extension length
	headerLen := binary.BigEndian.Uint16(frame[8:10])
	if headerLen != 0 {
		t.Fatalf("header_len: got %d, want 0", headerLen)
	}

	// Check payload length
	payloadLen := binary.BigEndian.Uint32(frame[10:14])
	if int(payloadLen) != len(payload) {
		t.Fatalf("payload_len: got %d, want %d", payloadLen, len(payload))
	}

	// Check CRC
	crcVal := binary.BigEndian.Uint32(frame[14:18])
	if crcVal != crc32c(payload) {
		t.Fatalf("crc: got 0x%08x, want 0x%08x", crcVal, crc32c(payload))
	}

	// Check payload bytes
	if !bytes.Equal(frame[18:], payload) {
		t.Fatalf("payload: got %q, want %q", frame[18:], payload)
	}
}

func TestDecodeFrameCRCMismatch(t *testing.T) {
	payload := []byte(`{"test":true}`)
	frame, err := encodeFrame(payload)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}

	// Corrupt a payload byte
	frame[len(frame)-1] ^= 0xFF

	_, err = decodeFrame(bytes.NewReader(frame))
	if err != ErrCRCMismatch {
		t.Fatalf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestDecodeFrameInvalidMagic(t *testing.T) {
	frame := make([]byte, frameHeaderSize)
	copy(frame[0:4], "XXXX")
	binary.BigEndian.PutUint16(frame[4:6], protocolVersion)

	_, err := decodeFrame(bytes.NewReader(frame))
	if err != ErrInvalidMagic {
		t.Fatalf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestDecodeFrameUnsupportedVersion(t *testing.T) {
	frame := make([]byte, frameHeaderSize)
	copy(frame[0:4], frameMagic)
	binary.BigEndian.PutUint16(frame[4:6], 99)

	_, err := decodeFrame(bytes.NewReader(frame))
	if err != ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestDecodeFrameInvalidFlags(t *testing.T) {
	frame := make([]byte, frameHeaderSize)
	copy(frame[0:4], frameMagic)
	binary.BigEndian.PutUint16(frame[4:6], protocolVersion)
	binary.BigEndian.PutUint16(frame[6:8], 0x0010) // Invalid flag bit

	_, err := decodeFrame(bytes.NewReader(frame))
	if err != ErrInvalidFlags {
		t.Fatalf("expected ErrInvalidFlags, got %v", err)
	}
}

func TestDecodeFrameOversized(t *testing.T) {
	frame := make([]byte, frameHeaderSize)
	copy(frame[0:4], frameMagic)
	binary.BigEndian.PutUint16(frame[4:6], protocolVersion)
	binary.BigEndian.PutUint16(frame[6:8], flagCRCPresent)
	binary.BigEndian.PutUint32(frame[10:14], maxPayloadSize+1)

	_, err := decodeFrame(bytes.NewReader(frame))
	if err != ErrFrameTooLarge {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestEncodeFrameOversized(t *testing.T) {
	payload := make([]byte, maxPayloadSize+1)
	_, err := encodeFrame(payload)
	if err != ErrFrameTooLarge {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestDecodeFrameEmptyPayload(t *testing.T) {
	frame, err := encodeFrame([]byte{})
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}

	decoded, err := decodeFrame(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("expected empty payload, got %d bytes", len(decoded))
	}
}

func TestEncodeJSONL(t *testing.T) {
	msg := map[string]string{"type": "request", "op": "PING"}
	data, err := encodeJSONL(msg)
	if err != nil {
		t.Fatalf("encodeJSONL: %v", err)
	}

	if data[len(data)-1] != '\n' {
		t.Fatal("JSONL must end with newline")
	}

	var parsed map[string]string
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["op"] != "PING" {
		t.Fatalf("op: got %q, want PING", parsed["op"])
	}
}

func TestStringOrSliceSingleString(t *testing.T) {
	data := []byte(`"active"`)
	var s StringOrSlice
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 1 || s[0] != "active" {
		t.Fatalf("expected [active], got %v", s)
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `"active"` {
		t.Fatalf("marshal: got %s, want \"active\"", out)
	}
}

func TestStringOrSliceMultiple(t *testing.T) {
	data := []byte(`["a","b","c"]`)
	var s StringOrSlice
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 3 || s[0] != "a" || s[1] != "b" || s[2] != "c" {
		t.Fatalf("expected [a b c], got %v", s)
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `["a","b","c"]` {
		t.Fatalf("marshal: got %s, want [\"a\",\"b\",\"c\"]", out)
	}
}

func TestStringOrSliceEmpty(t *testing.T) {
	data := []byte(`[]`)
	var s StringOrSlice
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 0 {
		t.Fatalf("expected empty, got %v", s)
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `[]` {
		t.Fatalf("marshal: got %s, want []", out)
	}
}

func TestCRC32C(t *testing.T) {
	// Known CRC32C test vector: "123456789" → 0xE3069283
	result := crc32c([]byte("123456789"))
	if result != 0xE3069283 {
		t.Fatalf("crc32c(\"123456789\"): got 0x%08X, want 0xE3069283", result)
	}
}
