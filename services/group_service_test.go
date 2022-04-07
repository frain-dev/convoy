package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

type fakeGroupRepo struct {
}

func stringArrayCompare(expected, candidate []string) bool {
	if len(candidate) != len(expected) {
		return false
	}
	for i := range candidate {
		if strings.EqualFold(expected[i], candidate[i]) != true {
			return false
		}
	}

	return true
}

func (*fakeGroupRepo) LoadGroups(c context.Context, f *datastore.GroupFilter) ([]*datastore.Group, error) {
	if stringArrayCompare([]string{"grace"}, f.Names) {
		return []*datastore.Group{}, nil
	}

	return nil, errors.New("failed to trim filter.Names")
}
func (*fakeGroupRepo) CreateGroup(context.Context, *datastore.Group) error {
	return nil
}
func (*fakeGroupRepo) UpdateGroup(context.Context, *datastore.Group) error {
	return nil
}
func (*fakeGroupRepo) DeleteGroup(ctx context.Context, uid string) error {
	return nil
}
func (*fakeGroupRepo) FetchGroupByID(context.Context, string) (*datastore.Group, error) {
	return nil, nil
}
func (*fakeGroupRepo) FetchGroupsByIDs(context.Context, []string) ([]datastore.Group, error) {
	return nil, nil
}

func Test_GetGroups(t *testing.T) {
	tts := []struct {
		Name      string
		AppRepo   datastore.ApplicationRepository
		GroupRepo datastore.GroupRepository
		WantErr   bool
		Filter    *datastore.GroupFilter
	}{
		{
			Name:      "trims-whitespaces-from-query",
			Filter:    &datastore.GroupFilter{Names: []string{" grace "}},
			GroupRepo: &fakeGroupRepo{},
		},
		{
			Name:      "trims-whitespaces-from-query-retains-value-if-no-whitespace",
			Filter:    &datastore.GroupFilter{Names: []string{"grace"}},
			GroupRepo: &fakeGroupRepo{},
		},
		{
			Name:      "trims-whitespaces-from-query-retains-case",
			Filter:    &datastore.GroupFilter{Names: []string{" GraCe "}},
			GroupRepo: &fakeGroupRepo{},
		},
	}

	for _, tt := range tts {
		var g GroupService = GroupService{
			appRepo:   tt.AppRepo,
			groupRepo: tt.GroupRepo,
		}

		t.Run(tt.Name, func(t *testing.T) {
			_, err := g.GetGroups(context.TODO(), tt.Filter)

			if tt.WantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

}
