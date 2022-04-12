// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager (interfaces: Interface)

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	client "sigs.k8s.io/controller-runtime/pkg/client"
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

// AlertingRules mocks base method.
func (m *MockInterface) AlertingRules() (map[string]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AlertingRules")
	ret0, _ := ret[0].(map[string]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AlertingRules indicates an expected call of AlertingRules.
func (mr *MockInterfaceMockRecorder) AlertingRules() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AlertingRules", reflect.TypeOf((*MockInterface)(nil).AlertingRules))
}

// Deploy mocks base method.
func (m *MockInterface) Deploy(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockInterfaceMockRecorder) Deploy(arg0 interface{}) *gomock.Call {
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
func (mr *MockInterfaceMockRecorder) Destroy(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockInterface)(nil).Destroy), arg0)
}

// ScrapeConfigs mocks base method.
func (m *MockInterface) ScrapeConfigs() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScrapeConfigs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ScrapeConfigs indicates an expected call of ScrapeConfigs.
func (mr *MockInterfaceMockRecorder) ScrapeConfigs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScrapeConfigs", reflect.TypeOf((*MockInterface)(nil).ScrapeConfigs))
}

// SetReplicaCount mocks base method.
func (m *MockInterface) SetReplicaCount(arg0 int32) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetReplicaCount", arg0)
}

// SetReplicaCount indicates an expected call of SetReplicaCount.
func (mr *MockInterfaceMockRecorder) SetReplicaCount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetReplicaCount", reflect.TypeOf((*MockInterface)(nil).SetReplicaCount), arg0)
}

// SetShootClient mocks base method.
func (m *MockInterface) SetShootClient(arg0 client.Client) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetShootClient", arg0)
}

// SetShootClient indicates an expected call of SetShootClient.
func (mr *MockInterfaceMockRecorder) SetShootClient(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetShootClient", reflect.TypeOf((*MockInterface)(nil).SetShootClient), arg0)
}

// Wait mocks base method.
func (m *MockInterface) Wait(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockInterfaceMockRecorder) Wait(arg0 interface{}) *gomock.Call {
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
func (mr *MockInterfaceMockRecorder) WaitCleanup(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockInterface)(nil).WaitCleanup), arg0)
}

// WaitForControllerToBeActive mocks base method.
func (m *MockInterface) WaitForControllerToBeActive(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForControllerToBeActive", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitForControllerToBeActive indicates an expected call of WaitForControllerToBeActive.
func (mr *MockInterfaceMockRecorder) WaitForControllerToBeActive(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForControllerToBeActive", reflect.TypeOf((*MockInterface)(nil).WaitForControllerToBeActive), arg0)
}
