package organisation_members

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

func seedNamedOrganisation(t *testing.T, db database.Database, ownerID, name string) *datastore.Organisation {
	t.Helper()

	org := &datastore.Organisation{
		UID:       ulid.Make().String(),
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := createOrganisationService(t, db).CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

func Test_CountUserOrganisations(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)
	orgService := createOrganisationService(t, db)

	user := seedUser(t, db, "")
	alpha := seedNamedOrganisation(t, db, user.UID, "Alpha Corp")
	beta := seedNamedOrganisation(t, db, user.UID, "Beta Inc")
	gamma := seedNamedOrganisation(t, db, user.UID, "Gamma LLC")

	_ = seedOrganisationMember(t, db, alpha.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, beta.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, gamma.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// unfiltered count returns every org the user belongs to.
	total, err := service.CountUserOrganisations(ctx, user.UID, "")
	require.NoError(t, err)
	require.Equal(t, int64(3), total)

	// name search is a case-insensitive partial match.
	total, err = service.CountUserOrganisations(ctx, user.UID, "alph")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// id search is an exact match.
	total, err = service.CountUserOrganisations(ctx, user.UID, beta.UID)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// search is consistent with the paged list filter.
	pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Search: "alph"}
	pageable.SetCursors()
	filtered, _, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "Alpha Corp", filtered[0].Name)

	// soft-deleted orgs are excluded from the count.
	require.NoError(t, orgService.DeleteOrganisation(ctx, alpha.UID))
	total, err = service.CountUserOrganisations(ctx, user.UID, "")
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
}
