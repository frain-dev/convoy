// Code generated by MockGen. DO NOT EDIT.
// Source: internal/pkg/tracer/tracer.go
//
// Generated by this command:
//
//	mockgen --source internal/pkg/tracer/tracer.go --destination mocks/tracer.go -package mocks
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	config "github.com/frain-dev/convoy/config"
	gomock "go.uber.org/mock/gomock"
)

// MockBackend is a mock of Backend interface.
type MockBackend struct {
	ctrl     *gomock.Controller
	recorder *MockBackendMockRecorder
}

// MockBackendMockRecorder is the mock recorder for MockBackend.
type MockBackendMockRecorder struct {
	mock *MockBackend
}

// NewMockBackend creates a new mock instance.
func NewMockBackend(ctrl *gomock.Controller) *MockBackend {
	mock := &MockBackend{ctrl: ctrl}
	mock.recorder = &MockBackendMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBackend) EXPECT() *MockBackendMockRecorder {
	return m.recorder
}

// Capture mocks base method.
func (m *MockBackend) Capture(ctx context.Context, name string, attributes map[string]any, startTime, endTime time.Time) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Capture", ctx, name, attributes, startTime, endTime)
}

// Capture indicates an expected call of Capture.
func (mr *MockBackendMockRecorder) Capture(ctx, name, attributes, startTime, endTime any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Capture", reflect.TypeOf((*MockBackend)(nil).Capture), ctx, name, attributes, startTime, endTime)
}

// Init mocks base method.
func (m *MockBackend) Init(componentName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", componentName)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockBackendMockRecorder) Init(componentName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockBackend)(nil).Init), componentName)
}

// Shutdown mocks base method.
func (m *MockBackend) Shutdown(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Shutdown", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Shutdown indicates an expected call of Shutdown.
func (mr *MockBackendMockRecorder) Shutdown(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Shutdown", reflect.TypeOf((*MockBackend)(nil).Shutdown), ctx)
}

// Type mocks base method.
func (m *MockBackend) Type() config.TracerProvider {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Type")
	ret0, _ := ret[0].(config.TracerProvider)
	return ret0
}

// Type indicates an expected call of Type.
func (mr *MockBackendMockRecorder) Type() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Type", reflect.TypeOf((*MockBackend)(nil).Type))
}
