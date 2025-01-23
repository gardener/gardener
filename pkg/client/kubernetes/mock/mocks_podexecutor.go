// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/client/kubernetes (interfaces: PodExecutor)
//
// Generated by this command:
//
//	mockgen -package mock -destination=mocks_podexecutor.go github.com/gardener/gardener/pkg/client/kubernetes PodExecutor
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	io "io"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockPodExecutor is a mock of PodExecutor interface.
type MockPodExecutor struct {
	ctrl     *gomock.Controller
	recorder *MockPodExecutorMockRecorder
	isgomock struct{}
}

// MockPodExecutorMockRecorder is the mock recorder for MockPodExecutor.
type MockPodExecutorMockRecorder struct {
	mock *MockPodExecutor
}

// NewMockPodExecutor creates a new mock instance.
func NewMockPodExecutor(ctrl *gomock.Controller) *MockPodExecutor {
	mock := &MockPodExecutor{ctrl: ctrl}
	mock.recorder = &MockPodExecutorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPodExecutor) EXPECT() *MockPodExecutorMockRecorder {
	return m.recorder
}

// Execute mocks base method.
func (m *MockPodExecutor) Execute(ctx context.Context, namespace, name, containerName string, command ...string) (io.Reader, io.Reader, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, namespace, name, containerName}
	for _, a := range command {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Execute", varargs...)
	ret0, _ := ret[0].(io.Reader)
	ret1, _ := ret[1].(io.Reader)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Execute indicates an expected call of Execute.
func (mr *MockPodExecutorMockRecorder) Execute(ctx, namespace, name, containerName any, command ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, namespace, name, containerName}, command...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockPodExecutor)(nil).Execute), varargs...)
}

// ExecuteWithStreams mocks base method.
func (m *MockPodExecutor) ExecuteWithStreams(ctx context.Context, namespace, name, containerName string, stdin io.Reader, stdout, stderr io.Writer, command ...string) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, namespace, name, containerName, stdin, stdout, stderr}
	for _, a := range command {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ExecuteWithStreams", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// ExecuteWithStreams indicates an expected call of ExecuteWithStreams.
func (mr *MockPodExecutorMockRecorder) ExecuteWithStreams(ctx, namespace, name, containerName, stdin, stdout, stderr any, command ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, namespace, name, containerName, stdin, stdout, stderr}, command...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteWithStreams", reflect.TypeOf((*MockPodExecutor)(nil).ExecuteWithStreams), varargs...)
}
