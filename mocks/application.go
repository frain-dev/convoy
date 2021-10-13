// Code generated by MockGen. DO NOT EDIT.
// Source: application.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	convoy "github.com/frain-dev/convoy"
	models "github.com/frain-dev/convoy/server/models"
	mongopagination "github.com/gobeam/mongo-go-pagination"
	gomock "github.com/golang/mock/gomock"
)

// MockApplicationRepository is a mock of ApplicationRepository interface.
type MockApplicationRepository struct {
	ctrl     *gomock.Controller
	recorder *MockApplicationRepositoryMockRecorder
}

// MockApplicationRepositoryMockRecorder is the mock recorder for MockApplicationRepository.
type MockApplicationRepositoryMockRecorder struct {
	mock *MockApplicationRepository
}

// NewMockApplicationRepository creates a new mock instance.
func NewMockApplicationRepository(ctrl *gomock.Controller) *MockApplicationRepository {
	mock := &MockApplicationRepository{ctrl: ctrl}
	mock.recorder = &MockApplicationRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockApplicationRepository) EXPECT() *MockApplicationRepositoryMockRecorder {
	return m.recorder
}

// CreateApplication mocks base method.
func (m *MockApplicationRepository) CreateApplication(arg0 context.Context, arg1 *convoy.Application) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateApplication", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateApplication indicates an expected call of CreateApplication.
func (mr *MockApplicationRepositoryMockRecorder) CreateApplication(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateApplication", reflect.TypeOf((*MockApplicationRepository)(nil).CreateApplication), arg0, arg1)
}

// DeleteApplication mocks base method.
func (m *MockApplicationRepository) DeleteApplication(arg0 context.Context, arg1 *convoy.Application) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteApplication", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteApplication indicates an expected call of DeleteApplication.
func (mr *MockApplicationRepositoryMockRecorder) DeleteApplication(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteApplication", reflect.TypeOf((*MockApplicationRepository)(nil).DeleteApplication), arg0, arg1)
}

// FindApplicationByID mocks base method.
func (m *MockApplicationRepository) FindApplicationByID(arg0 context.Context, arg1 string) (*convoy.Application, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindApplicationByID", arg0, arg1)
	ret0, _ := ret[0].(*convoy.Application)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindApplicationByID indicates an expected call of FindApplicationByID.
func (mr *MockApplicationRepositoryMockRecorder) FindApplicationByID(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindApplicationByID", reflect.TypeOf((*MockApplicationRepository)(nil).FindApplicationByID), arg0, arg1)
}

// FindApplicationEndpointByID mocks base method.
func (m *MockApplicationRepository) FindApplicationEndpointByID(arg0 context.Context, arg1, arg2 string) (*convoy.Endpoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindApplicationEndpointByID", arg0, arg1, arg2)
	ret0, _ := ret[0].(*convoy.Endpoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindApplicationEndpointByID indicates an expected call of FindApplicationEndpointByID.
func (mr *MockApplicationRepositoryMockRecorder) FindApplicationEndpointByID(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindApplicationEndpointByID", reflect.TypeOf((*MockApplicationRepository)(nil).FindApplicationEndpointByID), arg0, arg1, arg2)
}

// LoadApplicationsPaged mocks base method.
func (m *MockApplicationRepository) LoadApplicationsPaged(arg0 context.Context, arg1 string, arg2 models.Pageable) ([]convoy.Application, mongopagination.PaginationData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadApplicationsPaged", arg0, arg1, arg2)
	ret0, _ := ret[0].([]convoy.Application)
	ret1, _ := ret[1].(mongopagination.PaginationData)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// LoadApplicationsPaged indicates an expected call of LoadApplicationsPaged.
func (mr *MockApplicationRepositoryMockRecorder) LoadApplicationsPaged(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadApplicationsPaged", reflect.TypeOf((*MockApplicationRepository)(nil).LoadApplicationsPaged), arg0, arg1, arg2)
}

// LoadApplicationsPagedByGroupId mocks base method.
func (m *MockApplicationRepository) LoadApplicationsPagedByGroupId(arg0 context.Context, arg1 string, arg2 models.Pageable) ([]convoy.Application, mongopagination.PaginationData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadApplicationsPagedByGroupId", arg0, arg1, arg2)
	ret0, _ := ret[0].([]convoy.Application)
	ret1, _ := ret[1].(mongopagination.PaginationData)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// LoadApplicationsPagedByGroupId indicates an expected call of LoadApplicationsPagedByGroupId.
func (mr *MockApplicationRepositoryMockRecorder) LoadApplicationsPagedByGroupId(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadApplicationsPagedByGroupId", reflect.TypeOf((*MockApplicationRepository)(nil).LoadApplicationsPagedByGroupId), arg0, arg1, arg2)
}

// SearchApplicationsByGroupId mocks base method.
func (m *MockApplicationRepository) SearchApplicationsByGroupId(arg0 context.Context, arg1 string, arg2 models.SearchParams) ([]convoy.Application, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SearchApplicationsByGroupId", arg0, arg1, arg2)
	ret0, _ := ret[0].([]convoy.Application)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SearchApplicationsByGroupId indicates an expected call of SearchApplicationsByGroupId.
func (mr *MockApplicationRepositoryMockRecorder) SearchApplicationsByGroupId(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SearchApplicationsByGroupId", reflect.TypeOf((*MockApplicationRepository)(nil).SearchApplicationsByGroupId), arg0, arg1, arg2)
}

// UpdateApplication mocks base method.
func (m *MockApplicationRepository) UpdateApplication(arg0 context.Context, arg1 *convoy.Application) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateApplication", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateApplication indicates an expected call of UpdateApplication.
func (mr *MockApplicationRepositoryMockRecorder) UpdateApplication(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateApplication", reflect.TypeOf((*MockApplicationRepository)(nil).UpdateApplication), arg0, arg1)
}

// UpdateApplicationEndpointsStatus mocks base method.
func (m *MockApplicationRepository) UpdateApplicationEndpointsStatus(arg0 context.Context, arg1 string, arg2 []string, arg3 convoy.EndpointStatus) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateApplicationEndpointsStatus", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateApplicationEndpointsStatus indicates an expected call of UpdateApplicationEndpointsStatus.
func (mr *MockApplicationRepositoryMockRecorder) UpdateApplicationEndpointsStatus(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateApplicationEndpointsStatus", reflect.TypeOf((*MockApplicationRepository)(nil).UpdateApplicationEndpointsStatus), arg0, arg1, arg2, arg3)
}
