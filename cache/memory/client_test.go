package mcache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type data struct {
	Name string
}

const key = "test_key"

type MemoryCacheIntegrationTestSuite struct {
	suite.Suite
	cache *MemoryCache
}

func (s *MemoryCacheIntegrationTestSuite) SetupTest() {
	// Each test gets a fresh in-memory cache
	s.cache = NewMemoryCache()
}

func (s *MemoryCacheIntegrationTestSuite) Test_WriteToCache() {
	err := s.cache.Set(context.TODO(), key, &data{Name: "test_name"}, 10*time.Second)
	s.Require().NoError(err)
}

func (s *MemoryCacheIntegrationTestSuite) Test_ReadFromCache() {
	err := s.cache.Set(context.TODO(), key, &data{Name: "test_name"}, 10*time.Second)
	s.Require().NoError(err)

	var item data
	err = s.cache.Get(context.TODO(), key, &item)

	s.Require().NoError(err)
	s.Require().Equal("test_name", item.Name)
}

func (s *MemoryCacheIntegrationTestSuite) Test_DeleteFromCache() {
	err := s.cache.Set(context.TODO(), key, &data{Name: "test_name"}, 10*time.Second)
	s.Require().NoError(err)

	err = s.cache.Delete(context.TODO(), key)
	s.Require().NoError(err)

	var item data
	err = s.cache.Get(context.TODO(), key, &item)
	s.Require().NoError(err)

	s.Require().Equal("", item.Name)
}

func TestMemoryCacheIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(MemoryCacheIntegrationTestSuite))
}
