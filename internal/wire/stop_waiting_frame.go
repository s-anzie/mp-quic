package wire

import (
	"bytes"
	"errors"

	"github.com/s-anzie/mp-quic/internal/protocol"
	"github.com/s-anzie/mp-quic/internal/utils"
	"github.com/s-anzie/mp-quic/qerr"
)

// A StopWaitingFrame in QUIC
type StopWaitingFrame struct {
	LeastUnacked    protocol.PacketNumber
	PacketNumberLen protocol.PacketNumberLen
	// PacketNumber is the packet number of the packet that this StopWaitingFrame will be sent with
	PacketNumber protocol.PacketNumber
}

var (
	errLeastUnackedHigherThanPacketNumber = errors.New("StopWaitingFrame: LeastUnacked can't be greater than the packet number")
	errPacketNumberNotSet                 = errors.New("StopWaitingFrame: PacketNumber not set")
	errPacketNumberLenNotSet              = errors.New("StopWaitingFrame: PacketNumberLen not set")
)

func (f *StopWaitingFrame) Write(b *bytes.Buffer, version protocol.VersionNumber) error {
	// make sure the PacketNumber was set
	if f.PacketNumber == protocol.PacketNumber(0) {
		return errPacketNumberNotSet
	}
	if f.LeastUnacked > f.PacketNumber {
		return errLeastUnackedHigherThanPacketNumber
	}

	b.WriteByte(0x06)
	leastUnackedDelta := uint64(f.PacketNumber - f.LeastUnacked)
	switch f.PacketNumberLen {
	case protocol.PacketNumberLen1:
		b.WriteByte(uint8(leastUnackedDelta))
	case protocol.PacketNumberLen2:
		utils.GetByteOrder(version).WriteUint16(b, uint16(leastUnackedDelta))
	case protocol.PacketNumberLen4:
		utils.GetByteOrder(version).WriteUint32(b, uint32(leastUnackedDelta))
	case protocol.PacketNumberLen6:
		utils.GetByteOrder(version).WriteUint48(b, leastUnackedDelta&(1<<48-1))
	default:
		return errPacketNumberLenNotSet
	}
	return nil
}

// MinLength of a written frame
func (f *StopWaitingFrame) MinLength(version protocol.VersionNumber) (protocol.ByteCount, error) {
	minLength := protocol.ByteCount(1) // typeByte

	if f.PacketNumberLen == protocol.PacketNumberLenInvalid {
		return 0, errPacketNumberLenNotSet
	}
	minLength += protocol.ByteCount(f.PacketNumberLen)
	return minLength, nil
}

// ParseStopWaitingFrame parses a StopWaiting frame
func ParseStopWaitingFrame(r *bytes.Reader, packetNumber protocol.PacketNumber, packetNumberLen protocol.PacketNumberLen, version protocol.VersionNumber) (*StopWaitingFrame, error) {
	frame := &StopWaitingFrame{}

	// read the TypeByte
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}

	leastUnackedDelta, err := utils.GetByteOrder(version).ReadUintN(r, uint8(packetNumberLen))
	if err != nil {
		return nil, err
	}
	if leastUnackedDelta >= uint64(packetNumber) {
		return nil, qerr.Error(qerr.InvalidStopWaitingData, "invalid LeastUnackedDelta")
	}
	frame.LeastUnacked = protocol.PacketNumber(uint64(packetNumber) - leastUnackedDelta)
	return frame, nil
}
