package ackhandler

import (
	"time"

	"github.com/s-anzie/mp-quic/internal/protocol"
	"github.com/s-anzie/mp-quic/internal/wire"
)

// SentPacketHandler handles ACKs received for outgoing packets
type SentPacketHandler interface {
	// SentPacket may modify the packet
	SentPacket(packet *Packet) error
	ReceivedAck(ackFrame *wire.AckFrame, withPacketNumber protocol.PacketNumber, recvTime time.Time) error

	// Specific to multipath operation
	ReceivedClosePath(f *wire.ClosePathFrame, withPacketNumber protocol.PacketNumber, recvTime time.Time) error
	SetInflightAsLost()

	SendingAllowed() bool
	GetStopWaitingFrame(force bool) *wire.StopWaitingFrame
	ShouldSendRetransmittablePacket() bool
	DequeuePacketForRetransmission() (packet *Packet)
	GetLeastUnacked() protocol.PacketNumber

	GetAlarmTimeout() time.Time
	OnAlarm()

	DuplicatePacket(packet *Packet)

	GetStatistics() (uint64, uint64, uint64)
	GetlastSentPacketNumber() (protocol.PacketNumber, protocol.PacketNumber, protocol.PacketNumber, uint64)
	SetlastSentPacketNumber(lastsend uint64, largestReceive uint64, LargestAcked uint64, packet uint64)

	IncrementBytesInFlight(bytesInFlight protocol.ByteCount)
}

// ReceivedPacketHandler handles ACKs needed to send for incoming packets
type ReceivedPacketHandler interface {
	ReceivedPacket(packetNumber protocol.PacketNumber, shouldInstigateAck bool) error
	SetLowerLimit(protocol.PacketNumber)

	GetAlarmTimeout() time.Time
	GetAckFrame() *wire.AckFrame

	GetClosePathFrame() *wire.ClosePathFrame

	GetStatistics() uint64
	GetlargestlowerLimitpacketHistory() (protocol.PacketNumber, protocol.PacketNumber, uint64, *wire.AckFrame)
	SetRcvPacketHandler(uint64, uint64, uint64, uint64, uint64)
}
