package quic

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/haterb4/mp-quic/internal/flowcontrol"
	"github.com/haterb4/mp-quic/internal/protocol"
	"github.com/haterb4/mp-quic/internal/utils"
	"github.com/haterb4/mp-quic/internal/wire"
)

// A Stream assembles the data from StreamFrames and provides a super-convenient Read-Interface
//
// Read() and Write() may be called concurrently, but multiple calls to Read() or Write() individually must be synchronized manually.
type stream struct {
	mutex sync.Mutex

	ctx       context.Context
	ctxCancel context.CancelFunc

	streamID protocol.StreamID
	onData   func()
	// onReset is a callback that should send a RST_STREAM
	onReset func(protocol.StreamID, protocol.ByteCount)

	readPosInFrame int
	writeOffset    protocol.ByteCount
	readOffset     protocol.ByteCount

	// Once set, the errors must not be changed!
	err error

	// cancelled is set when Cancel() is called
	cancelled utils.AtomicBool
	// finishedReading is set once we read a frame with a FinBit
	finishedReading utils.AtomicBool
	// finisedWriting is set once Close() is called
	finishedWriting utils.AtomicBool
	// resetLocally is set if Reset() is called
	resetLocally utils.AtomicBool
	// resetRemotely is set if RegisterRemoteError() is called
	resetRemotely utils.AtomicBool

	frameQueue   *streamFrameSorter
	readChan     chan struct{}
	readDeadline time.Time

	dataForWriting []byte
	finSent        utils.AtomicBool
	rstSent        utils.AtomicBool
	writeChan      chan struct{}
	writeDeadline  time.Time

	flowControlManager flowcontrol.FlowControlManager
}
type receivedFrame struct {
	frame *wire.StreamFrame
	index int
}

var _ Stream = &stream{}

type deadlineError struct{}

func (deadlineError) Error() string   { return "deadline exceeded" }
func (deadlineError) Temporary() bool { return true }
func (deadlineError) Timeout() bool   { return true }

var errDeadline net.Error = &deadlineError{}

// newStream creates a new Stream
func newStream(StreamID protocol.StreamID,
	onData func(),
	onReset func(protocol.StreamID, protocol.ByteCount),
	flowControlManager flowcontrol.FlowControlManager) *stream {
	s := &stream{
		onData:             onData,
		onReset:            onReset,
		streamID:           StreamID,
		flowControlManager: flowControlManager,
		frameQueue:         newStreamFrameSorter(),
		readChan:           make(chan struct{}, 1),
		writeChan:          make(chan struct{}, 1),
	}
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	return s
}

// Read implements io.Reader. It is not thread safe!
func (s *stream) Read(p []byte) (int, error) {
	s.mutex.Lock()
	err := s.err
	s.mutex.Unlock()
	utils.Infof(" :)    1 %+v \n", s.readPosInFrame)
	if s.cancelled.Get() || s.resetLocally.Get() {
		utils.Infof(" :)    return  1 \n")
		return 0, err

	}
	if s.finishedReading.Get() {
		utils.Infof(" :)   return  2 \n")
		return 0, io.EOF

	}

	bytesRead := 0
	for bytesRead < len(p) {
		utils.Infof(" :)    2 \n")
		s.mutex.Lock()
		frame := s.frameQueue.Head()
		if frame == nil && bytesRead > 0 {
			err = s.err
			s.mutex.Unlock()
			utils.Infof(" :)   return  3 \n")
			return bytesRead, err
		}

		var err error
		for {
			utils.Infof(" :)    3 %+v\n", frame)
			// Stop waiting on errors
			if s.resetLocally.Get() || s.cancelled.Get() {
				err = s.err
				utils.Infof(" :)    break  1 \n")
				break
			}
			utils.Infof(" :)      4 \n")
			deadline := s.readDeadline
			if !deadline.IsZero() && !time.Now().Before(deadline) {
				err = errDeadline
				utils.Infof(" :)    break  2 \n")
				break
			}

			if frame != nil {
				s.readPosInFrame = int(s.readOffset - frame.Offset)
				utils.Infof(" :)    break  3 \n")
				break
			}
			utils.Infof(" :)      5 \n")
			s.mutex.Unlock()
			if deadline.IsZero() {
				utils.Infof(" :)      6 \n")
				<-s.readChan
			} else {
				utils.Infof(" :)      7 \n")
				select {
				case <-s.readChan:
				case <-time.After(deadline.Sub(time.Now())):
				}
			}
			utils.Infof(" :)    8 \n")
			s.mutex.Lock()
			frame = s.frameQueue.Head()
		}
		utils.Infof(" :)      9 \n")
		s.mutex.Unlock()

		if err != nil {
			utils.Infof(" :)    return  4 \n")
			return bytesRead, err
		}
		utils.Infof(" :)      10 \n")
		m := utils.Min(len(p)-bytesRead, int(frame.DataLen())-s.readPosInFrame)

		if bytesRead > len(p) {
			utils.Infof(" :)    return  5 \n")
			return bytesRead, fmt.Errorf("BUG: bytesRead (%d) > len(p) (%d) in stream.Read", bytesRead, len(p))
		}
		if s.readPosInFrame > int(frame.DataLen()) {
			utils.Infof(" :)    return  6 \n")
			return bytesRead, fmt.Errorf("BUG: readPosInFrame (%d) > frame.DataLen (%d) in stream.Read", s.readPosInFrame, frame.DataLen())
		}
		copy(p[bytesRead:], frame.Data[s.readPosInFrame:])
		utils.Infof(" :)      11 \n")
		s.readPosInFrame += m
		bytesRead += m
		s.readOffset += protocol.ByteCount(m)

		// when a RST_STREAM was received, the was already informed about the final byteOffset for this stream
		if !s.resetRemotely.Get() {
			utils.Infof(" :)      12 \n")
			s.flowControlManager.AddBytesRead(s.streamID, protocol.ByteCount(m))
		}
		s.onData() // so that a possible WINDOW_UPDATE is sent
		s.onDataCallback()
		utils.Infof(" :)      13 \n")
		if s.readPosInFrame >= int(frame.DataLen()) {
			utils.Infof(" :)      14 \n")
			fin := frame.FinBit
			s.mutex.Lock()
			s.frameQueue.Pop()
			s.mutex.Unlock()
			if fin {
				s.finishedReading.Set(true)
				utils.Infof(" :)    return  7 \n")
				return bytesRead, io.EOF
			}
		}
	}
	utils.Infof(" :)    return  15 \n")
	return bytesRead, nil
}

func (s *stream) ReadAvailable(p []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled.Get() || s.resetLocally.Get() {
		return 0, s.err
	}
	if s.finishedReading.Get() {
		return 0, io.EOF
	}

	bytesRead := 0
	for bytesRead < len(p) {
		frame := s.findAvailableFrame()
		if frame == nil {
			if bytesRead > 0 {
				return bytesRead, nil
			}
			if err := s.waitForData(); err != nil {
				return 0, err
			}
			continue
		}

		// Calculer combien de bytes nous pouvons lire de ce frame
		frameOffset := int(frame.Offset)
		readStart := utils.Max(int(s.readOffset-protocol.ByteCount(frameOffset)), 0)
		readEnd := utils.Min(int(protocol.ByteCount(len(frame.Data))), int(protocol.ByteCount(len(p)-bytesRead)+s.readOffset-protocol.ByteCount(frameOffset)))
		bytesToRead := int(readEnd - readStart)

		copy(p[bytesRead:], frame.Data[readStart:readEnd])
		bytesRead += bytesToRead
		s.readOffset += protocol.ByteCount(bytesToRead)

		if !s.resetRemotely.Get() {
			s.flowControlManager.AddBytesRead(s.streamID, protocol.ByteCount(bytesToRead))
		}
		s.onData()
		s.onDataCallback()

		if readEnd == int(protocol.ByteCount(len(frame.Data))) && frame.FinBit {
			s.finishedReading.Set(true)
			return bytesRead, io.EOF
		}
	}

	return bytesRead, nil
}

func (s *stream) findAvailableFrame() *wire.StreamFrame {
	for _, frame := range s.frameQueue.queuedFrames {
		if uint64(frame.Offset) <= uint64(s.readOffset) && uint64(frame.Offset)+uint64(len(frame.Data)) > uint64(s.readOffset) {
			return frame
		}
	}
	return nil
}

func (s *stream) waitForData() error {
	for {
		if s.cancelled.Get() || s.resetLocally.Get() {
			return s.err
		}

		deadline := s.readDeadline
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return errDeadline
		}

		s.mutex.Unlock()
		var timeout <-chan time.Time
		if !deadline.IsZero() {
			timeout = time.After(deadline.Sub(time.Now()))
		}
		select {
		case <-s.readChan:
			s.mutex.Lock()
			return nil
		case <-timeout:
			s.mutex.Lock()
			return errDeadline
		}
	}
}

func (s *stream) Write(p []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.resetLocally.Get() || s.err != nil {
		return 0, s.err
	}
	if s.finishedWriting.Get() {
		return 0, fmt.Errorf("write on closed stream %d", s.streamID)
	}
	if len(p) == 0 {
		return 0, nil
	}

	s.dataForWriting = make([]byte, len(p))
	copy(s.dataForWriting, p)
	s.onData()

	var err error
	for {
		deadline := s.writeDeadline
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			err = errDeadline
			break
		}
		if s.dataForWriting == nil || s.err != nil {
			break
		}

		s.mutex.Unlock()
		if deadline.IsZero() {
			<-s.writeChan
		} else {
			select {
			case <-s.writeChan:
			case <-time.After(deadline.Sub(time.Now())):
			}
		}
		s.mutex.Lock()
	}

	if err != nil {
		return 0, err
	}
	if s.err != nil {
		return len(p) - len(s.dataForWriting), s.err
	}
	return len(p), nil
}

func (s *stream) lenOfDataForWriting() protocol.ByteCount {
	s.mutex.Lock()
	var l protocol.ByteCount
	if s.err == nil {
		l = protocol.ByteCount(len(s.dataForWriting))
	}
	s.mutex.Unlock()
	return l
}

func (s *stream) getDataForWriting(maxBytes protocol.ByteCount) []byte {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.err != nil || s.dataForWriting == nil {
		return nil
	}

	var ret []byte
	if protocol.ByteCount(len(s.dataForWriting)) > maxBytes {
		ret = s.dataForWriting[:maxBytes]
		s.dataForWriting = s.dataForWriting[maxBytes:]
	} else {
		ret = s.dataForWriting
		s.dataForWriting = nil
		s.signalWrite()
	}
	s.writeOffset += protocol.ByteCount(len(ret))
	return ret
}

// Close implements io.Closer
func (s *stream) Close() error {
	s.finishedWriting.Set(true)
	s.ctxCancel()
	s.onData()
	return nil
}

func (s *stream) shouldSendReset() bool {
	if s.rstSent.Get() {
		return false
	}
	return (s.resetLocally.Get() || s.resetRemotely.Get()) && !s.finishedWriteAndSentFin()
}

func (s *stream) shouldSendFin() bool {
	s.mutex.Lock()
	res := s.finishedWriting.Get() && !s.finSent.Get() && s.err == nil && s.dataForWriting == nil
	s.mutex.Unlock()
	return res
}

func (s *stream) sentFin() {
	s.finSent.Set(true)
}

// AddStreamFrame adds a new stream frame
func (s *stream) AddStreamFrame(frame *wire.StreamFrame) error {
	utils.Infof(" (AddStreamFrame)      1 : %+v \n", frame)
	log.Println("Adding Stream Frame with offset", frame.Offset, "and data length", frame.DataLen())
	maxOffset := frame.Offset + frame.DataLen()
	err := s.flowControlManager.UpdateHighestReceived(s.streamID, maxOffset)
	utils.Infof(" (AddStreamFrame)      2 : %+v\n", maxOffset)
	if err != nil {
		utils.Infof(" (AddStreamFrame)      3 error  UpdateHighestReceived \n")
		return err
	}
	utils.Infof(" (AddStreamFrame)      4 Pushing Frame:\n")
	s.mutex.Lock()
	defer s.mutex.Unlock()
	err = s.frameQueue.Push(frame)

	if err != nil && err != errDuplicateStreamData {
		utils.Infof(" (AddStreamFrame)     5 Error While Pushing Frame :%+v\n", err)
		return err
	}
	utils.Infof(" (AddStreamFrame)      5 Frame Pushed \n")
	s.signalRead()
	return nil
}

// signalRead performs a non-blocking send on the readChan
func (s *stream) signalRead() {
	select {
	case s.readChan <- struct{}{}:
	default:
	}
}

// signalRead performs a non-blocking send on the writeChan
func (s *stream) signalWrite() {
	select {
	case s.writeChan <- struct{}{}:
	default:
	}
}

func (s *stream) SetReadDeadline(t time.Time) error {
	s.mutex.Lock()
	oldDeadline := s.readDeadline
	s.readDeadline = t
	s.mutex.Unlock()
	// if the new deadline is before the currently set deadline, wake up Read()
	if t.Before(oldDeadline) {
		s.signalRead()
	}
	return nil
}

func (s *stream) SetWriteDeadline(t time.Time) error {
	s.mutex.Lock()
	oldDeadline := s.writeDeadline
	s.writeDeadline = t
	s.mutex.Unlock()
	if t.Before(oldDeadline) {
		s.signalWrite()
	}
	return nil
}

func (s *stream) SetDeadline(t time.Time) error {
	_ = s.SetReadDeadline(t)  // SetReadDeadline never errors
	_ = s.SetWriteDeadline(t) // SetWriteDeadline never errors
	return nil
}

// CloseRemote makes the stream receive a "virtual" FIN stream frame at a given offset
func (s *stream) CloseRemote(offset protocol.ByteCount) {
	s.AddStreamFrame(&wire.StreamFrame{FinBit: true, Offset: offset})
}

// Cancel is called by session to indicate that an error occurred
// The stream should will be closed immediately
func (s *stream) Cancel(err error) {
	s.mutex.Lock()
	s.cancelled.Set(true)
	s.ctxCancel()
	// errors must not be changed!
	if s.err == nil {
		s.err = err
		s.signalRead()
		s.signalWrite()
	}
	s.mutex.Unlock()
}

// resets the stream locally
func (s *stream) Reset(err error) {
	if s.resetLocally.Get() {
		return
	}
	s.mutex.Lock()
	s.resetLocally.Set(true)
	s.ctxCancel()
	// errors must not be changed!
	if s.err == nil {
		s.err = err
		s.signalRead()
		s.signalWrite()
	}
	if s.shouldSendReset() {
		s.onReset(s.streamID, s.writeOffset)
		s.rstSent.Set(true)
	}
	s.mutex.Unlock()
}

// resets the stream remotely
func (s *stream) RegisterRemoteError(err error) {
	if s.resetRemotely.Get() {
		return
	}
	s.mutex.Lock()
	s.resetRemotely.Set(true)
	s.ctxCancel()
	// errors must not be changed!
	if s.err == nil {
		s.err = err
		s.signalWrite()
	}
	if s.shouldSendReset() {
		s.onReset(s.streamID, s.writeOffset)
		s.rstSent.Set(true)
	}
	s.mutex.Unlock()
}

func (s *stream) finishedWriteAndSentFin() bool {
	return s.finishedWriting.Get() && s.finSent.Get()
}

func (s *stream) finished() bool {
	return s.cancelled.Get() ||
		(s.finishedReading.Get() && s.finishedWriteAndSentFin()) ||
		(s.resetRemotely.Get() && s.rstSent.Get()) ||
		(s.finishedReading.Get() && s.rstSent.Get()) ||
		(s.finishedWriteAndSentFin() && s.resetRemotely.Get())
}

func (s *stream) Context() context.Context {
	return s.ctx
}

func (s *stream) StreamID() protocol.StreamID {
	return s.streamID
}

func (s *stream) GetBytesSent() (protocol.ByteCount, error) {
	return s.flowControlManager.GetBytesSent(s.streamID)
}

func (s *stream) GetBytesRetrans() (protocol.ByteCount, error) {
	return s.flowControlManager.GetBytesRetrans(s.streamID)
}
func (s *stream) GetReadPosInFrame() (int, uint64, uint64) {
	return s.readPosInFrame, uint64(s.readOffset), uint64(s.writeOffset)
}
func (s *stream) SetReadPosInFrame(readPosInFrame int) {
	s.readPosInFrame = readPosInFrame
}
func (s *stream) SetReadOffset(readOffset uint64) {
	s.readOffset = protocol.ByteCount(readOffset)
}
func (s *stream) Setuint64(writeOffset uint64) {
	s.writeOffset = protocol.ByteCount(writeOffset)
}

// Updated by haterb4
func (s *stream) IncrementReceiveWindow(increment uint64) {
	s.flowControlManager.IncrementReceiveWindow(s.streamID, protocol.ByteCount(increment))
	s.signalWrite() // Notify that the window size has been increased
	s.signalRead()
}

// added by haterb4
func (s *stream) onDataCallback() {
	// Calculate how much data has been received and how much window space is left
	receivedData := s.readOffset
	windowSize, err := s.flowControlManager.GetReceiveWindow(s.streamID)
	if err != nil {
		utils.Infof("Error getting receive window: %v", err)
		return
	}

	// If the window is almost full, increase the window size
	if receivedData >= protocol.ByteCount(int(float64(windowSize)*0.9)) {
		log.Println("Increasing window size from", windowSize, "to", windowSize*2, "as it is almost full", receivedData)
		windowSize *= 2
		s.IncrementReceiveWindow(uint64(windowSize))
		s.signalRead()
		s.signalWrite()
	}
}
