// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/component/gardener/resourcemanager (interfaces: Interface)
//
// Generated by this command:
//
//	mockgen -package mock -destination=mocks.go github.com/gardener/gardener/pkg/component/gardener/resourcemanager Interface
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	resourcemanager "github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	gomock "go.uber.org/mock/gomock"
)

// MockInterface is a mock of Interface interface.
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface.
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance.
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// Deploy mocks base method.
func (m *MockInterface) Deploy(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockInterfaceMockRecorder) Deploy(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockInterface)(nil).Deploy), arg0)
}

// Destroy mocks base method.
func (m *MockInterface) Destroy(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockInterfaceMockRecorder) Destroy(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockInterface)(nil).Destroy), arg0)
}

// GetReplicas mocks base method.
func (m *MockInterface) GetReplicas() *int32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetReplicas")
	ret0, _ := ret[0].(*int32)
	return ret0
}

// GetReplicas indicates an expected call of GetReplicas.
func (mr *MockInterfaceMockRecorder) GetReplicas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReplicas", reflect.TypeOf((*MockInterface)(nil).GetReplicas))
}

// GetValues mocks base method.
func (m *MockInterface) GetValues() resourcemanager.Values {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetValues")
	ret0, _ := ret[0].(resourcemanager.Values)
	return ret0
}

// GetValues indicates an expected call of GetValues.
func (mr *MockInterfaceMockRecorder) GetValues() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValues", reflect.TypeOf((*MockInterface)(nil).GetValues))
}

// SetReplicas mocks base method.
func (m *MockInterface) SetReplicas(arg0 *int32) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetReplicas", arg0)
}

// SetReplicas indicates an expected call of SetReplicas.
func (mr *MockInterfaceMockRecorder) SetReplicas(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetReplicas", reflect.TypeOf((*MockInterface)(nil).SetReplicas), arg0)
}

// SetSecrets mocks base method.
func (m *MockInterface) SetSecrets(arg0 resourcemanager.Secrets) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetSecrets", arg0)
}

// SetSecrets indicates an expected call of SetSecrets.
func (mr *MockInterfaceMockRecorder) SetSecrets(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSecrets", reflect.TypeOf((*MockInterface)(nil).SetSecrets), arg0)
}

// Wait mocks base method.
func (m *MockInterface) Wait(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockInterfaceMockRecorder) Wait(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockInterface)(nil).Wait), arg0)
}

// WaitCleanup mocks base method.
func (m *MockInterface) WaitCleanup(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitCleanup", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitCleanup indicates an expected call of WaitCleanup.
func (mr *MockInterfaceMockRecorder) WaitCleanup(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockInterface)(nil).WaitCleanup), arg0)
}
