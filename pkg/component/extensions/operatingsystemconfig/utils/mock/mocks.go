// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils (interfaces: UnitSerializer,FileContentInlineCodec)
//
// Generated by this command:
//
//	mockgen -package=utils -destination=mocks.go github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils UnitSerializer,FileContentInlineCodec
//

// Package utils is a generated GoMock package.
package utils

import (
	reflect "reflect"

	unit "github.com/coreos/go-systemd/v22/unit"
	v1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gomock "go.uber.org/mock/gomock"
)

// MockUnitSerializer is a mock of UnitSerializer interface.
type MockUnitSerializer struct {
	ctrl     *gomock.Controller
	recorder *MockUnitSerializerMockRecorder
	isgomock struct{}
}

// MockUnitSerializerMockRecorder is the mock recorder for MockUnitSerializer.
type MockUnitSerializerMockRecorder struct {
	mock *MockUnitSerializer
}

// NewMockUnitSerializer creates a new mock instance.
func NewMockUnitSerializer(ctrl *gomock.Controller) *MockUnitSerializer {
	mock := &MockUnitSerializer{ctrl: ctrl}
	mock.recorder = &MockUnitSerializerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUnitSerializer) EXPECT() *MockUnitSerializerMockRecorder {
	return m.recorder
}

// Deserialize mocks base method.
func (m *MockUnitSerializer) Deserialize(arg0 string) ([]*unit.UnitOption, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deserialize", arg0)
	ret0, _ := ret[0].([]*unit.UnitOption)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Deserialize indicates an expected call of Deserialize.
func (mr *MockUnitSerializerMockRecorder) Deserialize(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deserialize", reflect.TypeOf((*MockUnitSerializer)(nil).Deserialize), arg0)
}

// Serialize mocks base method.
func (m *MockUnitSerializer) Serialize(arg0 []*unit.UnitOption) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Serialize", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Serialize indicates an expected call of Serialize.
func (mr *MockUnitSerializerMockRecorder) Serialize(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Serialize", reflect.TypeOf((*MockUnitSerializer)(nil).Serialize), arg0)
}

// MockFileContentInlineCodec is a mock of FileContentInlineCodec interface.
type MockFileContentInlineCodec struct {
	ctrl     *gomock.Controller
	recorder *MockFileContentInlineCodecMockRecorder
	isgomock struct{}
}

// MockFileContentInlineCodecMockRecorder is the mock recorder for MockFileContentInlineCodec.
type MockFileContentInlineCodecMockRecorder struct {
	mock *MockFileContentInlineCodec
}

// NewMockFileContentInlineCodec creates a new mock instance.
func NewMockFileContentInlineCodec(ctrl *gomock.Controller) *MockFileContentInlineCodec {
	mock := &MockFileContentInlineCodec{ctrl: ctrl}
	mock.recorder = &MockFileContentInlineCodecMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFileContentInlineCodec) EXPECT() *MockFileContentInlineCodecMockRecorder {
	return m.recorder
}

// Decode mocks base method.
func (m *MockFileContentInlineCodec) Decode(arg0 *v1alpha1.FileContentInline) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Decode indicates an expected call of Decode.
func (mr *MockFileContentInlineCodecMockRecorder) Decode(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockFileContentInlineCodec)(nil).Decode), arg0)
}

// Encode mocks base method.
func (m *MockFileContentInlineCodec) Encode(arg0 []byte, arg1 string) (*v1alpha1.FileContentInline, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", arg0, arg1)
	ret0, _ := ret[0].(*v1alpha1.FileContentInline)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Encode indicates an expected call of Encode.
func (mr *MockFileContentInlineCodecMockRecorder) Encode(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockFileContentInlineCodec)(nil).Encode), arg0, arg1)
}
