// Code generated by MockGen. DO NOT EDIT.
// Source: sigs.k8s.io/controller-runtime/pkg/cache (interfaces: Cache)
//
// Generated by this command:
//
//	mockgen -package cache -destination=mocks.go sigs.k8s.io/controller-runtime/pkg/cache Cache
//

// Package cache is a generated GoMock package.
package cache

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "sigs.k8s.io/controller-runtime/pkg/cache"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockCache is a mock of Cache interface.
type MockCache struct {
	ctrl     *gomock.Controller
	recorder *MockCacheMockRecorder
	isgomock struct{}
}

// MockCacheMockRecorder is the mock recorder for MockCache.
type MockCacheMockRecorder struct {
	mock *MockCache
}

// NewMockCache creates a new mock instance.
func NewMockCache(ctrl *gomock.Controller) *MockCache {
	mock := &MockCache{ctrl: ctrl}
	mock.recorder = &MockCacheMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCache) EXPECT() *MockCacheMockRecorder {
	return m.recorder
}

// Get mocks base method.
func (m *MockCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, key, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Get", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *MockCacheMockRecorder) Get(ctx, key, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, key, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockCache)(nil).Get), varargs...)
}

// GetInformer mocks base method.
func (m *MockCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetInformer", varargs...)
	ret0, _ := ret[0].(cache.Informer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInformer indicates an expected call of GetInformer.
func (mr *MockCacheMockRecorder) GetInformer(ctx, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInformer", reflect.TypeOf((*MockCache)(nil).GetInformer), varargs...)
}

// GetInformerForKind mocks base method.
func (m *MockCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, gvk}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetInformerForKind", varargs...)
	ret0, _ := ret[0].(cache.Informer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInformerForKind indicates an expected call of GetInformerForKind.
func (mr *MockCacheMockRecorder) GetInformerForKind(ctx, gvk any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, gvk}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInformerForKind", reflect.TypeOf((*MockCache)(nil).GetInformerForKind), varargs...)
}

// IndexField mocks base method.
func (m *MockCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IndexField", ctx, obj, field, extractValue)
	ret0, _ := ret[0].(error)
	return ret0
}

// IndexField indicates an expected call of IndexField.
func (mr *MockCacheMockRecorder) IndexField(ctx, obj, field, extractValue any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IndexField", reflect.TypeOf((*MockCache)(nil).IndexField), ctx, obj, field, extractValue)
}

// List mocks base method.
func (m *MockCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, list}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "List", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// List indicates an expected call of List.
func (mr *MockCacheMockRecorder) List(ctx, list any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, list}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockCache)(nil).List), varargs...)
}

// RemoveInformer mocks base method.
func (m *MockCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveInformer", ctx, obj)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveInformer indicates an expected call of RemoveInformer.
func (mr *MockCacheMockRecorder) RemoveInformer(ctx, obj any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveInformer", reflect.TypeOf((*MockCache)(nil).RemoveInformer), ctx, obj)
}

// Start mocks base method.
func (m *MockCache) Start(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockCacheMockRecorder) Start(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockCache)(nil).Start), ctx)
}

// WaitForCacheSync mocks base method.
func (m *MockCache) WaitForCacheSync(ctx context.Context) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForCacheSync", ctx)
	ret0, _ := ret[0].(bool)
	return ret0
}

// WaitForCacheSync indicates an expected call of WaitForCacheSync.
func (mr *MockCacheMockRecorder) WaitForCacheSync(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForCacheSync", reflect.TypeOf((*MockCache)(nil).WaitForCacheSync), ctx)
}
