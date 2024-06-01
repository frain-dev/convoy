// Code generated by MockGen. DO NOT EDIT.
// Source: internal/pkg/memorystore/table.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	memorystore "github.com/frain-dev/convoy/internal/pkg/memorystore"
	gomock "github.com/golang/mock/gomock"
)

// MockITable is a mock of ITable interface.
type MockITable struct {
	ctrl     *gomock.Controller
	recorder *MockITableMockRecorder
}

// MockITableMockRecorder is the mock recorder for MockITable.
type MockITableMockRecorder struct {
	mock *MockITable
}

// NewMockITable creates a new mock instance.
func NewMockITable(ctrl *gomock.Controller) *MockITable {
	mock := &MockITable{ctrl: ctrl}
	mock.recorder = &MockITableMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockITable) EXPECT() *MockITableMockRecorder {
	return m.recorder
}

// GetItems mocks base method.
func (m *MockITable) GetItems(prefix string) []*memorystore.Row {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetItems", prefix)
	ret0, _ := ret[0].([]*memorystore.Row)
	return ret0
}

// GetItems indicates an expected call of GetItems.
func (mr *MockITableMockRecorder) GetItems(prefix interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetItems", reflect.TypeOf((*MockITable)(nil).GetItems), prefix)
}

// MockSyncer is a mock of Syncer interface.
type MockSyncer struct {
	ctrl     *gomock.Controller
	recorder *MockSyncerMockRecorder
}

// MockSyncerMockRecorder is the mock recorder for MockSyncer.
type MockSyncerMockRecorder struct {
	mock *MockSyncer
}

// NewMockSyncer creates a new mock instance.
func NewMockSyncer(ctrl *gomock.Controller) *MockSyncer {
	mock := &MockSyncer{ctrl: ctrl}
	mock.recorder = &MockSyncerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSyncer) EXPECT() *MockSyncerMockRecorder {
	return m.recorder
}

// SyncChanges mocks base method.
func (m *MockSyncer) SyncChanges(arg0 context.Context, arg1 *memorystore.Table) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncChanges", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncChanges indicates an expected call of SyncChanges.
func (mr *MockSyncerMockRecorder) SyncChanges(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncChanges", reflect.TypeOf((*MockSyncer)(nil).SyncChanges), arg0, arg1)
}
