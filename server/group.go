package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createGroupService(a *ApplicationHandler) *services.GroupService {
	apiKeyRepo := mongo.NewApiKeyRepo(a.A.Store)
	groupRepo := mongo.NewGroupRepo(a.A.Store)
	eventRepo := mongo.NewEventRepository(a.A.Store)
	eventDeliveryRepo := mongo.NewEventDeliveryRepository(a.A.Store)

	return services.NewGroupService(
		apiKeyRepo, groupRepo, eventRepo,
		eventDeliveryRepo, a.A.Limiter, a.A.Cache,
	)
}

// GetGroup - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Get a project
// @Description This endpoint fetches a project by its id
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Success 200 {object} util.ServerResponse{data=datastore.Group}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID} [get]
func _() {}

func (a *ApplicationHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	groupService := createGroupService(a)

	err := groupService.FillGroupsStatistics(r.Context(), []*datastore.Group{group})
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Group fetched successfully",
		group, http.StatusOK))
}

// DeleteGroup - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Delete a project
// @Description This endpoint deletes a project using its id
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID} [delete]
func _() {}

func (a *ApplicationHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	groupService := createGroupService(a)

	//opts := &policies.GroupPolicyOpts{
	//	OrganisationRepo:       mongo.NewOrgRepo(a.A.Store),
	//	OrganisationMemberRepo: mongo.NewOrgMemberRepo(a.A.Store),
	//}
	//gp := policies.NewGroupPolicy(opts)
	//if err := gp.Delete(r.Context(), group); err != nil {
	//	_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
	//	return
	//}

	err := groupService.DeleteGroup(r.Context(), group.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Group deleted successfully",
		nil, http.StatusOK))
}

// CreateGroup - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Create a project
// @Description This endpoint creates a project
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param orgID query string true "Organisation id"
// @Param group body models.Group true "Group Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Group}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects [post]
func _() {}

func (a *ApplicationHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var newGroup models.Group
	err := util.ReadJSON(r, &newGroup)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	org := m.GetOrganisationFromContext(r.Context())
	member := m.GetOrganisationMemberFromContext(r.Context())
	groupService := createGroupService(a)

	group, apiKey, err := groupService.CreateGroup(r.Context(), &newGroup, org, member)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.CreateGroupResponse{
		APIKey: apiKey,
		Group:  group,
	}

	_ = render.Render(w, r, util.NewServerResponse("Group created successfully", resp, http.StatusCreated))
}

// UpdateGroup - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Update a project
// @Description This endpoint updates a project
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param group body models.Group true "Group Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Group}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID} [put]
func _() {}

func (a *ApplicationHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateGroup
	err := util.ReadJSON(r, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	g := m.GetGroupFromContext(r.Context())
	groupService := createGroupService(a)

	group, err := groupService.UpdateGroup(r.Context(), g, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Group updated successfully", group, http.StatusAccepted))
}

// GetGroups - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Get projects
// @Description This endpoint fetches projects
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param name query string false "group name"
// @Param orgID query string true "organisation id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.Group}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects [get]
func _() {}

func (a *ApplicationHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	org := m.GetOrganisationFromContext(r.Context())
	name := r.URL.Query().Get("name")

	filter := &datastore.GroupFilter{OrgID: org.UID}
	filter.Names = append(filter.Names, name)
	groupService := createGroupService(a)

	groups, err := groupService.GetGroups(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Groups fetched successfully", groups, http.StatusOK))
}
