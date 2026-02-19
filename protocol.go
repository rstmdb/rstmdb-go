package rstmdb

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	frameMagic      = "RCPX"
	frameHeaderSize = 18
	protocolVersion = uint16(1)
	maxPayloadSize  = 16 * 1024 * 1024 // 16 MiB

	flagCRCPresent = uint16(0x0001)
	flagCompressed = uint16(0x0002)
	flagStream     = uint16(0x0004)
	flagEndStream  = uint16(0x0008)
	flagValidMask  = uint16(0x000F)
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func crc32c(data []byte) uint32 {
	return crc32.Checksum(data, crc32cTable)
}

// encodeFrame builds an 18-byte RCPX header + payload. CRC_PRESENT is always set.
func encodeFrame(payload []byte) ([]byte, error) {
	plen := len(payload)
	if plen > maxPayloadSize {
		return nil, ErrFrameTooLarge
	}

	frame := make([]byte, frameHeaderSize+plen)

	// Magic
	copy(frame[0:4], frameMagic)
	// Version
	binary.BigEndian.PutUint16(frame[4:6], protocolVersion)
	// Flags (CRC_PRESENT)
	binary.BigEndian.PutUint16(frame[6:8], flagCRCPresent)
	// Header extension length (0)
	binary.BigEndian.PutUint16(frame[8:10], 0)
	// Payload length
	binary.BigEndian.PutUint32(frame[10:14], uint32(plen))
	// CRC32C of payload
	binary.BigEndian.PutUint32(frame[14:18], crc32c(payload))

	copy(frame[frameHeaderSize:], payload)
	return frame, nil
}

// decodeFrame reads an RCPX frame from r, validates the header and CRC, and returns the payload.
func decodeFrame(r io.Reader) ([]byte, error) {
	var hdr [frameHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionClosed, err)
	}

	// Validate magic
	if string(hdr[0:4]) != frameMagic {
		return nil, ErrInvalidMagic
	}

	// Validate version
	version := binary.BigEndian.Uint16(hdr[4:6])
	if version != protocolVersion {
		return nil, ErrUnsupportedVersion
	}

	// Validate flags
	flags := binary.BigEndian.Uint16(hdr[6:8])
	if flags & ^flagValidMask != 0 {
		return nil, ErrInvalidFlags
	}

	headerLen := binary.BigEndian.Uint16(hdr[8:10])
	payloadLen := binary.BigEndian.Uint32(hdr[10:14])
	frameCRC := binary.BigEndian.Uint32(hdr[14:18])

	if payloadLen > uint32(maxPayloadSize) {
		return nil, ErrFrameTooLarge
	}

	// Skip header extension if present
	if headerLen > 0 {
		if _, err := io.ReadFull(r, make([]byte, headerLen)); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConnectionClosed, err)
		}
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConnectionClosed, err)
		}
	}

	// Validate CRC if present
	if flags&flagCRCPresent != 0 {
		if crc32c(payload) != frameCRC {
			return nil, ErrCRCMismatch
		}
	}

	return payload, nil
}

// encodeJSONL marshals v as JSON followed by a newline.
func encodeJSONL(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
