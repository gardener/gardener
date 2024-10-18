// Code generated by MockGen. DO NOT EDIT.
// Source: k8s.io/client-go/tools/record (interfaces: EventRecorder)
//
// Generated by this command:
//
//	mockgen -package record -destination=mocks.go k8s.io/client-go/tools/record EventRecorder
//

// Package record is a generated GoMock package.
package record

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// MockEventRecorder is a mock of EventRecorder interface.
type MockEventRecorder struct {
	ctrl     *gomock.Controller
	recorder *MockEventRecorderMockRecorder
	isgomock struct{}
}

// MockEventRecorderMockRecorder is the mock recorder for MockEventRecorder.
type MockEventRecorderMockRecorder struct {
	mock *MockEventRecorder
}

// NewMockEventRecorder creates a new mock instance.
func NewMockEventRecorder(ctrl *gomock.Controller) *MockEventRecorder {
	mock := &MockEventRecorder{ctrl: ctrl}
	mock.recorder = &MockEventRecorderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventRecorder) EXPECT() *MockEventRecorderMockRecorder {
	return m.recorder
}

// AnnotatedEventf mocks base method.
func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...any) {
	m.ctrl.T.Helper()
	varargs := []any{object, annotations, eventtype, reason, messageFmt}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "AnnotatedEventf", varargs...)
}

// AnnotatedEventf indicates an expected call of AnnotatedEventf.
func (mr *MockEventRecorderMockRecorder) AnnotatedEventf(object, annotations, eventtype, reason, messageFmt any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{object, annotations, eventtype, reason, messageFmt}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AnnotatedEventf", reflect.TypeOf((*MockEventRecorder)(nil).AnnotatedEventf), varargs...)
}

// Event mocks base method.
func (m *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Event", object, eventtype, reason, message)
}

// Event indicates an expected call of Event.
func (mr *MockEventRecorderMockRecorder) Event(object, eventtype, reason, message any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Event", reflect.TypeOf((*MockEventRecorder)(nil).Event), object, eventtype, reason, message)
}

// Eventf mocks base method.
func (m *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...any) {
	m.ctrl.T.Helper()
	varargs := []any{object, eventtype, reason, messageFmt}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Eventf", varargs...)
}

// Eventf indicates an expected call of Eventf.
func (mr *MockEventRecorderMockRecorder) Eventf(object, eventtype, reason, messageFmt any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{object, eventtype, reason, messageFmt}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Eventf", reflect.TypeOf((*MockEventRecorder)(nil).Eventf), varargs...)
}
