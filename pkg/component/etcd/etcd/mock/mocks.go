// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/component/etcd/etcd (interfaces: Interface)
//
// Generated by this command:
//
//	mockgen -package mock -destination=mocks.go github.com/gardener/gardener/pkg/component/etcd/etcd Interface
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	v1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	etcd "github.com/gardener/gardener/pkg/component/etcd/etcd"
	gomock "go.uber.org/mock/gomock"
	rest "k8s.io/client-go/rest"
)

// MockInterface is a mock of Interface interface.
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
	isgomock struct{}
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
func (m *MockInterface) Deploy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockInterfaceMockRecorder) Deploy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockInterface)(nil).Deploy), ctx)
}

// Destroy mocks base method.
func (m *MockInterface) Destroy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockInterfaceMockRecorder) Destroy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockInterface)(nil).Destroy), ctx)
}

// Get mocks base method.
func (m *MockInterface) Get(arg0 context.Context) (*v1alpha1.Etcd, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(*v1alpha1.Etcd)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockInterfaceMockRecorder) Get(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockInterface)(nil).Get), arg0)
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
func (m *MockInterface) GetValues() etcd.Values {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetValues")
	ret0, _ := ret[0].(etcd.Values)
	return ret0
}

// GetValues indicates an expected call of GetValues.
func (mr *MockInterfaceMockRecorder) GetValues() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValues", reflect.TypeOf((*MockInterface)(nil).GetValues))
}

// RolloutPeerCA mocks base method.
func (m *MockInterface) RolloutPeerCA(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RolloutPeerCA", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RolloutPeerCA indicates an expected call of RolloutPeerCA.
func (mr *MockInterfaceMockRecorder) RolloutPeerCA(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RolloutPeerCA", reflect.TypeOf((*MockInterface)(nil).RolloutPeerCA), arg0)
}

// Scale mocks base method.
func (m *MockInterface) Scale(arg0 context.Context, arg1 int32) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Scale", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Scale indicates an expected call of Scale.
func (mr *MockInterfaceMockRecorder) Scale(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Scale", reflect.TypeOf((*MockInterface)(nil).Scale), arg0, arg1)
}

// SetBackupConfig mocks base method.
func (m *MockInterface) SetBackupConfig(config *etcd.BackupConfig) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetBackupConfig", config)
}

// SetBackupConfig indicates an expected call of SetBackupConfig.
func (mr *MockInterfaceMockRecorder) SetBackupConfig(config any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBackupConfig", reflect.TypeOf((*MockInterface)(nil).SetBackupConfig), config)
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

// Snapshot mocks base method.
func (m *MockInterface) Snapshot(arg0 context.Context, arg1 rest.HTTPClient) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Snapshot indicates an expected call of Snapshot.
func (mr *MockInterfaceMockRecorder) Snapshot(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockInterface)(nil).Snapshot), arg0, arg1)
}

// Wait mocks base method.
func (m *MockInterface) Wait(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockInterfaceMockRecorder) Wait(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockInterface)(nil).Wait), ctx)
}

// WaitCleanup mocks base method.
func (m *MockInterface) WaitCleanup(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitCleanup", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitCleanup indicates an expected call of WaitCleanup.
func (mr *MockInterfaceMockRecorder) WaitCleanup(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockInterface)(nil).WaitCleanup), ctx)
}
