// Code generated by MockGen. DO NOT EDIT.
// Source: ../flowcontrol/interface.go

package mocks_fc

import (
	gomock "github.com/golang/mock/gomock"
	"github.com/s-anzie/mp-quic/internal/flowcontrol"
	protocol "github.com/s-anzie/mp-quic/internal/protocol"
)

// MockFlowControlManager is a mock of FlowControlManager interface
type MockFlowControlManager struct {
	ctrl     *gomock.Controller
	recorder *MockFlowControlManagerMockRecorder
}

// MockFlowControlManagerMockRecorder is the mock recorder for MockFlowControlManager
type MockFlowControlManagerMockRecorder struct {
	mock *MockFlowControlManager
}

// NewMockFlowControlManager creates a new mock instance
func NewMockFlowControlManager(ctrl *gomock.Controller) *MockFlowControlManager {
	mock := &MockFlowControlManager{ctrl: ctrl}
	mock.recorder = &MockFlowControlManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockFlowControlManager) EXPECT() *MockFlowControlManagerMockRecorder {
	return _m.recorder
}

// NewStream mocks base method
func (_m *MockFlowControlManager) NewStream(streamID protocol.StreamID, contributesToConnectionFlow bool) {
	_m.ctrl.Call(_m, "NewStream", streamID, contributesToConnectionFlow)
}

// NewStream indicates an expected call of NewStream
func (_mr *MockFlowControlManagerMockRecorder) NewStream(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "NewStream", arg0, arg1)
}

// RemoveStream mocks base method
func (_m *MockFlowControlManager) RemoveStream(streamID protocol.StreamID) {
	_m.ctrl.Call(_m, "RemoveStream", streamID)
}

// RemoveStream indicates an expected call of RemoveStream
func (_mr *MockFlowControlManagerMockRecorder) RemoveStream(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "RemoveStream", arg0)
}

// ResetStream mocks base method
func (_m *MockFlowControlManager) ResetStream(streamID protocol.StreamID, byteOffset protocol.ByteCount) error {
	ret := _m.ctrl.Call(_m, "ResetStream", streamID, byteOffset)
	ret0, _ := ret[0].(error)
	return ret0
}

// ResetStream indicates an expected call of ResetStream
func (_mr *MockFlowControlManagerMockRecorder) ResetStream(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ResetStream", arg0, arg1)
}

// UpdateHighestReceived mocks base method
func (_m *MockFlowControlManager) UpdateHighestReceived(streamID protocol.StreamID, byteOffset protocol.ByteCount) error {
	ret := _m.ctrl.Call(_m, "UpdateHighestReceived", streamID, byteOffset)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateHighestReceived indicates an expected call of UpdateHighestReceived
func (_mr *MockFlowControlManagerMockRecorder) UpdateHighestReceived(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "UpdateHighestReceived", arg0, arg1)
}

// AddBytesRead mocks base method
func (_m *MockFlowControlManager) AddBytesRead(streamID protocol.StreamID, n protocol.ByteCount) error {
	ret := _m.ctrl.Call(_m, "AddBytesRead", streamID, n)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddBytesRead indicates an expected call of AddBytesRead
func (_mr *MockFlowControlManagerMockRecorder) AddBytesRead(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "AddBytesRead", arg0, arg1)
}

// GetWindowUpdates mocks base method
func (_m *MockFlowControlManager) GetWindowUpdates(force bool) []flowcontrol.WindowUpdate {
	ret := _m.ctrl.Call(_m, "GetWindowUpdates", force)
	ret0, _ := ret[0].([]flowcontrol.WindowUpdate)
	return ret0
}

// GetWindowUpdates indicates an expected call of GetWindowUpdates
func (_mr *MockFlowControlManagerMockRecorder) GetWindowUpdates(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetWindowUpdates", arg0)
}

// GetReceiveWindow mocks base method
func (_m *MockFlowControlManager) GetReceiveWindow(streamID protocol.StreamID) (protocol.ByteCount, error) {
	ret := _m.ctrl.Call(_m, "GetReceiveWindow", streamID)
	ret0, _ := ret[0].(protocol.ByteCount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetReceiveWindow indicates an expected call of GetReceiveWindow
func (_mr *MockFlowControlManagerMockRecorder) GetReceiveWindow(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetReceiveWindow", arg0)
}

// AddBytesSent mocks base method
func (_m *MockFlowControlManager) AddBytesSent(streamID protocol.StreamID, n protocol.ByteCount) error {
	ret := _m.ctrl.Call(_m, "AddBytesSent", streamID, n)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddBytesSent indicates an expected call of AddBytesSent
func (_mr *MockFlowControlManagerMockRecorder) AddBytesSent(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "AddBytesSent", arg0, arg1)
}

// SendWindowSize mocks base method
func (_m *MockFlowControlManager) SendWindowSize(streamID protocol.StreamID) (protocol.ByteCount, error) {
	ret := _m.ctrl.Call(_m, "SendWindowSize", streamID)
	ret0, _ := ret[0].(protocol.ByteCount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendWindowSize indicates an expected call of SendWindowSize
func (_mr *MockFlowControlManagerMockRecorder) SendWindowSize(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "SendWindowSize", arg0)
}

// RemainingConnectionWindowSize mocks base method
func (_m *MockFlowControlManager) RemainingConnectionWindowSize() protocol.ByteCount {
	ret := _m.ctrl.Call(_m, "RemainingConnectionWindowSize")
	ret0, _ := ret[0].(protocol.ByteCount)
	return ret0
}

// RemainingConnectionWindowSize indicates an expected call of RemainingConnectionWindowSize
func (_mr *MockFlowControlManagerMockRecorder) RemainingConnectionWindowSize() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "RemainingConnectionWindowSize")
}

// UpdateWindow mocks base method
func (_m *MockFlowControlManager) UpdateWindow(streamID protocol.StreamID, offset protocol.ByteCount) (bool, error) {
	ret := _m.ctrl.Call(_m, "UpdateWindow", streamID, offset)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateWindow indicates an expected call of UpdateWindow
func (_mr *MockFlowControlManagerMockRecorder) UpdateWindow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "UpdateWindow", arg0, arg1)
}

// GetBytesSent mocks base method
func (_m *MockFlowControlManager) GetBytesSent(streamID protocol.StreamID) (protocol.ByteCount, error) {
	ret := _m.ctrl.Call(_m, "GetBytesSent", streamID)
	ret0, _ := ret[0].(protocol.ByteCount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBytesSent indicates an expected call of GetBytesSent
func (_mr *MockFlowControlManagerMockRecorder) GetBytesSent(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetBytesSent", arg0)
}

// AddBytesRetrans mocks base method
func (_m *MockFlowControlManager) AddBytesRetrans(streamID protocol.StreamID, n protocol.ByteCount) error {
	ret := _m.ctrl.Call(_m, "AddBytesRetrans", streamID, n)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddBytesRetrans indicates an expected call of AddBytesRetrans
func (_mr *MockFlowControlManagerMockRecorder) AddBytesRetrans(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "AddBytesRetrans", arg0, arg1)
}

// GetBytesRetrans mocks base method
func (_m *MockFlowControlManager) GetBytesRetrans(streamID protocol.StreamID) (protocol.ByteCount, error) {
	ret := _m.ctrl.Call(_m, "GetBytesRetrans", streamID)
	ret0, _ := ret[0].(protocol.ByteCount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBytesRetrans indicates an expected call of GetBytesRetrans
func (_mr *MockFlowControlManagerMockRecorder) GetBytesRetrans(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetBytesRetrans", arg0)
}
