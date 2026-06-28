package main

import "encoding/binary"

const (
	screenWireVersion      = 1
	screenWireCodecJPEG    = 1
	screenWireCodecWebP    = 2
	screenWireCodecPNG     = 3
	screenWireCodecRGBA    = 4
	screenWireKindKeyframe = 1
	screenWireKindPatch    = 2
)

func encodeScreenWireFrame(frame screenFrame) []byte {
	payloadLength := len(frame.Data)
	packet := make([]byte, 36+payloadLength)
	copy(packet[0:4], []byte{'U', 'S', 'C', 'R'})
	packet[4] = screenWireVersion
	packet[5] = frame.Codec
	packet[6] = frame.Kind
	packet[7] = 0
	binary.BigEndian.PutUint32(packet[8:12], uint32(frame.X))
	binary.BigEndian.PutUint32(packet[12:16], uint32(frame.Y))
	binary.BigEndian.PutUint32(packet[16:20], uint32(frame.Width))
	binary.BigEndian.PutUint32(packet[20:24], uint32(frame.Height))
	binary.BigEndian.PutUint32(packet[24:28], uint32(frame.FullWidth))
	binary.BigEndian.PutUint32(packet[28:32], uint32(frame.FullHeight))
	binary.BigEndian.PutUint32(packet[32:36], uint32(payloadLength))
	copy(packet[36:], frame.Data)
	return packet
}
