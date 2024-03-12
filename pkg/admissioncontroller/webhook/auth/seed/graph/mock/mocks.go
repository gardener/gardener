// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph (interfaces: Interface)
//
// Generated by this command:
//
//	mockgen -package mock -destination=mocks.go github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph Interface
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	graph "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph"
	gomock "go.uber.org/mock/gomock"
	cache "sigs.k8s.io/controller-runtime/pkg/cache"
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

// HasPathFrom mocks base method.
func (m *MockInterface) HasPathFrom(arg0 graph.VertexType, arg1, arg2 string, arg3 graph.VertexType, arg4, arg5 string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasPathFrom", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasPathFrom indicates an expected call of HasPathFrom.
func (mr *MockInterfaceMockRecorder) HasPathFrom(arg0, arg1, arg2, arg3, arg4, arg5 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasPathFrom", reflect.TypeOf((*MockInterface)(nil).HasPathFrom), arg0, arg1, arg2, arg3, arg4, arg5)
}

// HasVertex mocks base method.
func (m *MockInterface) HasVertex(arg0 graph.VertexType, arg1, arg2 string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasVertex", arg0, arg1, arg2)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasVertex indicates an expected call of HasVertex.
func (mr *MockInterfaceMockRecorder) HasVertex(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasVertex", reflect.TypeOf((*MockInterface)(nil).HasVertex), arg0, arg1, arg2)
}

// Setup mocks base method.
func (m *MockInterface) Setup(arg0 context.Context, arg1 cache.Cache) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Setup", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Setup indicates an expected call of Setup.
func (mr *MockInterfaceMockRecorder) Setup(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Setup", reflect.TypeOf((*MockInterface)(nil).Setup), arg0, arg1)
}