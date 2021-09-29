// Code generated by MockGen. DO NOT EDIT.
// Source: message.go

// Package mock_convoy is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	convoy "github.com/frain-dev/convoy"
	models "github.com/frain-dev/convoy/server/models"
	mongopagination "github.com/gobeam/mongo-go-pagination"
	gomock "github.com/golang/mock/gomock"
)

// MockMessageRepository is a mock of MessageRepository interface.
type MockMessageRepository struct {
	ctrl     *gomock.Controller
	recorder *MockMessageRepositoryMockRecorder
}

// MockMessageRepositoryMockRecorder is the mock recorder for MockMessageRepository.
type MockMessageRepositoryMockRecorder struct {
	mock *MockMessageRepository
}

// NewMockMessageRepository creates a new mock instance.
func NewMockMessageRepository(ctrl *gomock.Controller) *MockMessageRepository {
	mock := &MockMessageRepository{ctrl: ctrl}
	mock.recorder = &MockMessageRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMessageRepository) EXPECT() *MockMessageRepositoryMockRecorder {
	return m.recorder
}

// CreateMessage mocks base method.
func (m *MockMessageRepository) CreateMessage(arg0 context.Context, arg1 *convoy.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateMessage", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateMessage indicates an expected call of CreateMessage.
func (mr *MockMessageRepositoryMockRecorder) CreateMessage(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateMessage", reflect.TypeOf((*MockMessageRepository)(nil).CreateMessage), arg0, arg1)
}

// FindMessageByID mocks base method.
func (m *MockMessageRepository) FindMessageByID(ctx context.Context, id string) (*convoy.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindMessageByID", ctx, id)
	ret0, _ := ret[0].(*convoy.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindMessageByID indicates an expected call of FindMessageByID.
func (mr *MockMessageRepositoryMockRecorder) FindMessageByID(ctx, id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindMessageByID", reflect.TypeOf((*MockMessageRepository)(nil).FindMessageByID), ctx, id)
}

// LoadAbandonedMessagesForPostingRetry mocks base method.
func (m *MockMessageRepository) LoadAbandonedMessagesForPostingRetry(arg0 context.Context) ([]convoy.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadAbandonedMessagesForPostingRetry", arg0)
	ret0, _ := ret[0].([]convoy.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadAbandonedMessagesForPostingRetry indicates an expected call of LoadAbandonedMessagesForPostingRetry.
func (mr *MockMessageRepositoryMockRecorder) LoadAbandonedMessagesForPostingRetry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadAbandonedMessagesForPostingRetry", reflect.TypeOf((*MockMessageRepository)(nil).LoadAbandonedMessagesForPostingRetry), arg0)
}

// LoadMessageIntervals mocks base method.
func (m *MockMessageRepository) LoadMessageIntervals(arg0 context.Context, arg1 string, arg2 models.SearchParams, arg3 convoy.Period, arg4 int) ([]models.MessageInterval, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadMessageIntervals", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].([]models.MessageInterval)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadMessageIntervals indicates an expected call of LoadMessageIntervals.
func (mr *MockMessageRepositoryMockRecorder) LoadMessageIntervals(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadMessageIntervals", reflect.TypeOf((*MockMessageRepository)(nil).LoadMessageIntervals), arg0, arg1, arg2, arg3, arg4)
}

// LoadMessagesForPostingRetry mocks base method.
func (m *MockMessageRepository) LoadMessagesForPostingRetry(arg0 context.Context) ([]convoy.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadMessagesForPostingRetry", arg0)
	ret0, _ := ret[0].([]convoy.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadMessagesForPostingRetry indicates an expected call of LoadMessagesForPostingRetry.
func (mr *MockMessageRepositoryMockRecorder) LoadMessagesForPostingRetry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadMessagesForPostingRetry", reflect.TypeOf((*MockMessageRepository)(nil).LoadMessagesForPostingRetry), arg0)
}

// LoadMessagesPaged mocks base method.
func (m *MockMessageRepository) LoadMessagesPaged(arg0 context.Context, arg1, arg2 string, arg3 models.SearchParams, arg4 models.Pageable) ([]convoy.Message, mongopagination.PaginationData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadMessagesPaged", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].([]convoy.Message)
	ret1, _ := ret[1].(mongopagination.PaginationData)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// LoadMessagesPaged indicates an expected call of LoadMessagesPaged.
func (mr *MockMessageRepositoryMockRecorder) LoadMessagesPaged(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadMessagesPaged", reflect.TypeOf((*MockMessageRepository)(nil).LoadMessagesPaged), arg0, arg1, arg2, arg3, arg4)
}

// LoadMessagesPagedByAppId mocks base method.
func (m *MockMessageRepository) LoadMessagesPagedByAppId(arg0 context.Context, arg1 string, arg2 models.SearchParams, arg3 models.Pageable) ([]convoy.Message, mongopagination.PaginationData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadMessagesPagedByAppId", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([]convoy.Message)
	ret1, _ := ret[1].(mongopagination.PaginationData)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// LoadMessagesPagedByAppId indicates an expected call of LoadMessagesPagedByAppId.
func (mr *MockMessageRepositoryMockRecorder) LoadMessagesPagedByAppId(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadMessagesPagedByAppId", reflect.TypeOf((*MockMessageRepository)(nil).LoadMessagesPagedByAppId), arg0, arg1, arg2, arg3)
}

// LoadMessagesScheduledForPosting mocks base method.
func (m *MockMessageRepository) LoadMessagesScheduledForPosting(arg0 context.Context) ([]convoy.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadMessagesScheduledForPosting", arg0)
	ret0, _ := ret[0].([]convoy.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadMessagesScheduledForPosting indicates an expected call of LoadMessagesScheduledForPosting.
func (mr *MockMessageRepositoryMockRecorder) LoadMessagesScheduledForPosting(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadMessagesScheduledForPosting", reflect.TypeOf((*MockMessageRepository)(nil).LoadMessagesScheduledForPosting), arg0)
}

// UpdateMessageWithAttempt mocks base method.
func (m_2 *MockMessageRepository) UpdateMessageWithAttempt(ctx context.Context, m convoy.Message, attempt convoy.MessageAttempt) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "UpdateMessageWithAttempt", ctx, m, attempt)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateMessageWithAttempt indicates an expected call of UpdateMessageWithAttempt.
func (mr *MockMessageRepositoryMockRecorder) UpdateMessageWithAttempt(ctx, m, attempt interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMessageWithAttempt", reflect.TypeOf((*MockMessageRepository)(nil).UpdateMessageWithAttempt), ctx, m, attempt)
}

// UpdateStatusOfMessages mocks base method.
func (m *MockMessageRepository) UpdateStatusOfMessages(arg0 context.Context, arg1 []convoy.Message, arg2 convoy.MessageStatus) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateStatusOfMessages", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateStatusOfMessages indicates an expected call of UpdateStatusOfMessages.
func (mr *MockMessageRepositoryMockRecorder) UpdateStatusOfMessages(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateStatusOfMessages", reflect.TypeOf((*MockMessageRepository)(nil).UpdateStatusOfMessages), arg0, arg1, arg2)
}
