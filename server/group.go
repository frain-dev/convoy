package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

// GetGroup
// @Summary Get a group
// @Description This endpoint fetches a group by its id
// @Tags Group
// @Accept  json
// @Produce  json
// @Param groupID path string true "Group id"
// @Success 200 {object} serverResponse{data=datastore.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups/{groupID} [get]
func (a *applicationHandler) GetGroup(w http.ResponseWriter, r *http.Request) {

	group := getGroupFromContext(r.Context())

	err := a.groupService.FillGroupStatistics(r.Context(), group)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err, http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Group fetched successfully",
		group, http.StatusOK))
}

// DeleteGroup
// @Summary Delete a group
// @Description This endpoint deletes a group using its id
// @Tags Group
// @Accept  json
// @Produce  json
// @Param groupID path string true "Group id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups/{groupID} [delete]
func (a *applicationHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	group := getGroupFromContext(r.Context())

	err := a.groupService.DeleteGroup(r.Context(), group.UID)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err, http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Group deleted successfully",
		nil, http.StatusOK))
}

// CreateGroup
// @Summary Create a group
// @Description This endpoint creates a group
// @Tags Group
// @Accept  json
// @Produce  json
// @Param group body models.Group true "Group Details"
// @Success 200 {object} serverResponse{data=datastore.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups [post]
func (a *applicationHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {

	var newGroup models.Group
	err := util.ReadJSON(r, &newGroup)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group, err := a.groupService.CreateGroup(r.Context(), &newGroup)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err, http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, newServerResponse("Group created successfully", group, http.StatusCreated))
}

// UpdateGroup
// @Summary Update a group
// @Description This endpoint updates a group
// @Tags Group
// @Accept  json
// @Produce  json
// @Param groupID path string true "group id"
// @Param group body models.Group true "Group Details"
// @Success 200 {object} serverResponse{data=datastore.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups/{groupID} [put]
func (a *applicationHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	var update models.Group
	err := util.ReadJSON(r, &update)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group, err := a.groupService.UpdateGroup(r.Context(), chi.URLParam(r, "groupID"), &update)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err, http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, newServerResponse("Group updated successfully", group, http.StatusAccepted))
}

// GetGroups
// @Summary Get groups
// @Description This endpoint fetches groups
// @Tags Group
// @Accept  json
// @Produce  json
// @Param name query string false "group name"
// @Success 200 {object} serverResponse{data=[]datastore.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups [get]
func (a *applicationHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	user := getAuthUserFromContext(r.Context())
	name := r.URL.Query().Get("name")
	userGroups := user.Role.Groups

	var filter *datastore.GroupFilter

	if !util.IsStringEmpty(name) {
		for _, g := range userGroups {
			if name == g {
				filter = &datastore.GroupFilter{Names: []string{name}}
				break
			}
		}

		if filter == nil {
			_ = render.Render(w, r, newErrorResponse("invalid group access", http.StatusForbidden))
			return
		}
	} else if user.Role.Type == auth.RoleSuperUser {
		filter = &datastore.GroupFilter{}
	} else {
		filter = &datastore.GroupFilter{Names: userGroups}
	}

	groups, err := a.groupService.GetGroups(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err, http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, newServerResponse("Groups fetched successfully", groups, http.StatusOK))
}
