// Code generated by MockGen. DO NOT EDIT.
// Source: rate_limiting_queue.go
//
// Generated by this command:
//
//	mockgen -package workqueue -destination=mocks.go -source=rate_limiting_queue.go TypedRateLimitingInterface
//

// Package workqueue is a generated GoMock package.
package workqueue

import (
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"
)

// MockTypedRateLimitingInterface is a mock of TypedRateLimitingInterface interface.
type MockTypedRateLimitingInterface[T comparable] struct {
	ctrl     *gomock.Controller
	recorder *MockTypedRateLimitingInterfaceMockRecorder[T]
	isgomock struct{}
}

// MockTypedRateLimitingInterfaceMockRecorder is the mock recorder for MockTypedRateLimitingInterface.
type MockTypedRateLimitingInterfaceMockRecorder[T comparable] struct {
	mock *MockTypedRateLimitingInterface[T]
}

// NewMockTypedRateLimitingInterface creates a new mock instance.
func NewMockTypedRateLimitingInterface[T comparable](ctrl *gomock.Controller) *MockTypedRateLimitingInterface[T] {
	mock := &MockTypedRateLimitingInterface[T]{ctrl: ctrl}
	mock.recorder = &MockTypedRateLimitingInterfaceMockRecorder[T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTypedRateLimitingInterface[T]) EXPECT() *MockTypedRateLimitingInterfaceMockRecorder[T] {
	return m.recorder
}

// Add mocks base method.
func (m *MockTypedRateLimitingInterface[T]) Add(item T) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Add", item)
}

// Add indicates an expected call of Add.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) Add(item any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).Add), item)
}

// AddAfter mocks base method.
func (m *MockTypedRateLimitingInterface[T]) AddAfter(item T, duration time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddAfter", item, duration)
}

// AddAfter indicates an expected call of AddAfter.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) AddAfter(item, duration any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddAfter", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).AddAfter), item, duration)
}

// AddRateLimited mocks base method.
func (m *MockTypedRateLimitingInterface[T]) AddRateLimited(item T) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddRateLimited", item)
}

// AddRateLimited indicates an expected call of AddRateLimited.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) AddRateLimited(item any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRateLimited", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).AddRateLimited), item)
}

// Done mocks base method.
func (m *MockTypedRateLimitingInterface[T]) Done(item T) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Done", item)
}

// Done indicates an expected call of Done.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) Done(item any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Done", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).Done), item)
}

// Forget mocks base method.
func (m *MockTypedRateLimitingInterface[T]) Forget(item T) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Forget", item)
}

// Forget indicates an expected call of Forget.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) Forget(item any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Forget", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).Forget), item)
}

// Get mocks base method.
func (m *MockTypedRateLimitingInterface[T]) Get() (T, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get")
	ret0, _ := ret[0].(T)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) Get() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).Get))
}

// Len mocks base method.
func (m *MockTypedRateLimitingInterface[T]) Len() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Len")
	ret0, _ := ret[0].(int)
	return ret0
}

// Len indicates an expected call of Len.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) Len() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Len", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).Len))
}

// NumRequeues mocks base method.
func (m *MockTypedRateLimitingInterface[T]) NumRequeues(item T) int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NumRequeues", item)
	ret0, _ := ret[0].(int)
	return ret0
}

// NumRequeues indicates an expected call of NumRequeues.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) NumRequeues(item any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NumRequeues", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).NumRequeues), item)
}

// ShutDown mocks base method.
func (m *MockTypedRateLimitingInterface[T]) ShutDown() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ShutDown")
}

// ShutDown indicates an expected call of ShutDown.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) ShutDown() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ShutDown", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).ShutDown))
}

// ShutDownWithDrain mocks base method.
func (m *MockTypedRateLimitingInterface[T]) ShutDownWithDrain() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ShutDownWithDrain")
}

// ShutDownWithDrain indicates an expected call of ShutDownWithDrain.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) ShutDownWithDrain() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ShutDownWithDrain", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).ShutDownWithDrain))
}

// ShuttingDown mocks base method.
func (m *MockTypedRateLimitingInterface[T]) ShuttingDown() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ShuttingDown")
	ret0, _ := ret[0].(bool)
	return ret0
}

// ShuttingDown indicates an expected call of ShuttingDown.
func (mr *MockTypedRateLimitingInterfaceMockRecorder[T]) ShuttingDown() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ShuttingDown", reflect.TypeOf((*MockTypedRateLimitingInterface[T])(nil).ShuttingDown))
}
