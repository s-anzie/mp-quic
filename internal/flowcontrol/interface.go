package flowcontrol

import "github.com/s-anzie/mp-quic/internal/protocol"

// WindowUpdate provides the data for WindowUpdateFrames.
type WindowUpdate struct {
	StreamID protocol.StreamID
	Offset   protocol.ByteCount
}

// A FlowControlManager manages the flow control
type FlowControlManager interface {
	NewStream(streamID protocol.StreamID, contributesToConnectionFlow bool)
	RemoveStream(streamID protocol.StreamID)
	// methods needed for receiving data
	ResetStream(streamID protocol.StreamID, byteOffset protocol.ByteCount) error
	UpdateHighestReceived(streamID protocol.StreamID, byteOffset protocol.ByteCount) error
	AddBytesRead(streamID protocol.StreamID, n protocol.ByteCount) error
	GetWindowUpdates(force bool) []WindowUpdate
	GetReceiveWindow(streamID protocol.StreamID) (protocol.ByteCount, error)
	// methods needed for sending data
	AddBytesSent(streamID protocol.StreamID, n protocol.ByteCount) error
	SendWindowSize(streamID protocol.StreamID) (protocol.ByteCount, error)
	RemainingConnectionWindowSize() protocol.ByteCount
	UpdateWindow(streamID protocol.StreamID, offset protocol.ByteCount) (bool, error)
	// methods useful to collect statistics
	GetBytesSent(streamID protocol.StreamID) (protocol.ByteCount, error)
	AddBytesRetrans(streamID protocol.StreamID, n protocol.ByteCount) error
	GetBytesRetrans(streamID protocol.StreamID) (protocol.ByteCount, error)
	//Augmenter la fenetre de reception
	IncrementReceiveWindow(streamID protocol.StreamID, incremente protocol.ByteCount)
}
