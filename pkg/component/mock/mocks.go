// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/component (interfaces: Deployer,Waiter,DeployWaiter,DeployMigrateWaiter)
//
// Generated by this command:
//
//	mockgen -package mock -destination=mocks.go github.com/gardener/gardener/pkg/component Deployer,Waiter,DeployWaiter,DeployMigrateWaiter
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gomock "go.uber.org/mock/gomock"
)

// MockDeployer is a mock of Deployer interface.
type MockDeployer struct {
	ctrl     *gomock.Controller
	recorder *MockDeployerMockRecorder
	isgomock struct{}
}

// MockDeployerMockRecorder is the mock recorder for MockDeployer.
type MockDeployerMockRecorder struct {
	mock *MockDeployer
}

// NewMockDeployer creates a new mock instance.
func NewMockDeployer(ctrl *gomock.Controller) *MockDeployer {
	mock := &MockDeployer{ctrl: ctrl}
	mock.recorder = &MockDeployerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeployer) EXPECT() *MockDeployerMockRecorder {
	return m.recorder
}

// Deploy mocks base method.
func (m *MockDeployer) Deploy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockDeployerMockRecorder) Deploy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockDeployer)(nil).Deploy), ctx)
}

// Destroy mocks base method.
func (m *MockDeployer) Destroy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockDeployerMockRecorder) Destroy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockDeployer)(nil).Destroy), ctx)
}

// MockWaiter is a mock of Waiter interface.
type MockWaiter struct {
	ctrl     *gomock.Controller
	recorder *MockWaiterMockRecorder
	isgomock struct{}
}

// MockWaiterMockRecorder is the mock recorder for MockWaiter.
type MockWaiterMockRecorder struct {
	mock *MockWaiter
}

// NewMockWaiter creates a new mock instance.
func NewMockWaiter(ctrl *gomock.Controller) *MockWaiter {
	mock := &MockWaiter{ctrl: ctrl}
	mock.recorder = &MockWaiterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWaiter) EXPECT() *MockWaiterMockRecorder {
	return m.recorder
}

// Wait mocks base method.
func (m *MockWaiter) Wait(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockWaiterMockRecorder) Wait(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockWaiter)(nil).Wait), ctx)
}

// WaitCleanup mocks base method.
func (m *MockWaiter) WaitCleanup(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitCleanup", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitCleanup indicates an expected call of WaitCleanup.
func (mr *MockWaiterMockRecorder) WaitCleanup(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockWaiter)(nil).WaitCleanup), ctx)
}

// MockDeployWaiter is a mock of DeployWaiter interface.
type MockDeployWaiter struct {
	ctrl     *gomock.Controller
	recorder *MockDeployWaiterMockRecorder
	isgomock struct{}
}

// MockDeployWaiterMockRecorder is the mock recorder for MockDeployWaiter.
type MockDeployWaiterMockRecorder struct {
	mock *MockDeployWaiter
}

// NewMockDeployWaiter creates a new mock instance.
func NewMockDeployWaiter(ctrl *gomock.Controller) *MockDeployWaiter {
	mock := &MockDeployWaiter{ctrl: ctrl}
	mock.recorder = &MockDeployWaiterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeployWaiter) EXPECT() *MockDeployWaiterMockRecorder {
	return m.recorder
}

// Deploy mocks base method.
func (m *MockDeployWaiter) Deploy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockDeployWaiterMockRecorder) Deploy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockDeployWaiter)(nil).Deploy), ctx)
}

// Destroy mocks base method.
func (m *MockDeployWaiter) Destroy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockDeployWaiterMockRecorder) Destroy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockDeployWaiter)(nil).Destroy), ctx)
}

// Wait mocks base method.
func (m *MockDeployWaiter) Wait(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockDeployWaiterMockRecorder) Wait(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockDeployWaiter)(nil).Wait), ctx)
}

// WaitCleanup mocks base method.
func (m *MockDeployWaiter) WaitCleanup(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitCleanup", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitCleanup indicates an expected call of WaitCleanup.
func (mr *MockDeployWaiterMockRecorder) WaitCleanup(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockDeployWaiter)(nil).WaitCleanup), ctx)
}

// MockDeployMigrateWaiter is a mock of DeployMigrateWaiter interface.
type MockDeployMigrateWaiter struct {
	ctrl     *gomock.Controller
	recorder *MockDeployMigrateWaiterMockRecorder
	isgomock struct{}
}

// MockDeployMigrateWaiterMockRecorder is the mock recorder for MockDeployMigrateWaiter.
type MockDeployMigrateWaiterMockRecorder struct {
	mock *MockDeployMigrateWaiter
}

// NewMockDeployMigrateWaiter creates a new mock instance.
func NewMockDeployMigrateWaiter(ctrl *gomock.Controller) *MockDeployMigrateWaiter {
	mock := &MockDeployMigrateWaiter{ctrl: ctrl}
	mock.recorder = &MockDeployMigrateWaiterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeployMigrateWaiter) EXPECT() *MockDeployMigrateWaiterMockRecorder {
	return m.recorder
}

// Deploy mocks base method.
func (m *MockDeployMigrateWaiter) Deploy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockDeployMigrateWaiterMockRecorder) Deploy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).Deploy), ctx)
}

// Destroy mocks base method.
func (m *MockDeployMigrateWaiter) Destroy(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockDeployMigrateWaiterMockRecorder) Destroy(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).Destroy), ctx)
}

// Migrate mocks base method.
func (m *MockDeployMigrateWaiter) Migrate(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Migrate", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Migrate indicates an expected call of Migrate.
func (mr *MockDeployMigrateWaiterMockRecorder) Migrate(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Migrate", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).Migrate), ctx)
}

// Restore mocks base method.
func (m *MockDeployMigrateWaiter) Restore(ctx context.Context, shootState *v1beta1.ShootState) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Restore", ctx, shootState)
	ret0, _ := ret[0].(error)
	return ret0
}

// Restore indicates an expected call of Restore.
func (mr *MockDeployMigrateWaiterMockRecorder) Restore(ctx, shootState any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Restore", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).Restore), ctx, shootState)
}

// Wait mocks base method.
func (m *MockDeployMigrateWaiter) Wait(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockDeployMigrateWaiterMockRecorder) Wait(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).Wait), ctx)
}

// WaitCleanup mocks base method.
func (m *MockDeployMigrateWaiter) WaitCleanup(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitCleanup", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitCleanup indicates an expected call of WaitCleanup.
func (mr *MockDeployMigrateWaiterMockRecorder) WaitCleanup(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).WaitCleanup), ctx)
}

// WaitMigrate mocks base method.
func (m *MockDeployMigrateWaiter) WaitMigrate(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitMigrate", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitMigrate indicates an expected call of WaitMigrate.
func (mr *MockDeployMigrateWaiterMockRecorder) WaitMigrate(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitMigrate", reflect.TypeOf((*MockDeployMigrateWaiter)(nil).WaitMigrate), ctx)
}
