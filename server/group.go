package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetGroup
// @Summary Get a group
// @Description This endpoint fetches a group by its id
// @Tags Group
// @Accept  json
// @Produce  json
// @Param groupID path string true "Group id"
// @Success 200 {object} serverResponse{data=convoy.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups/{groupID} [get]
func (a *applicationHandler) GetGroup(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Group fetched successfully",
		*getGroupFromContext(r.Context()), http.StatusOK))
}

// CreateGroup
// @Summary Create a group
// @Description This endpoint creates a group
// @Tags Group
// @Accept  json
// @Produce  json
// @Param group body models.Group true "Group Details"
// @Success 200 {object} serverResponse{data=convoy.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups [post]
func (a *applicationHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {

	var newGroup models.Group
	err := json.NewDecoder(r.Body).Decode(&newGroup)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	groupName := newGroup.Name
	if util.IsStringEmpty(groupName) {
		_ = render.Render(w, r, newErrorResponse("please provide a valid name", http.StatusBadRequest))
		return
	}

	group := &convoy.Group{
		UID:            uuid.New().String(),
		Name:           groupName,
		Config:         &newGroup.Config,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = a.groupRepo.CreateGroup(r.Context(), group)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating Group", http.StatusInternalServerError))
		return
	}

	// register task.
	taskName := convoy.EventProcessor.SetPrefix(groupName)
	task.CreateTask(taskName, *group, task.ProcessEventDelivery(a.appRepo, a.eventDeliveryRepo, a.groupRepo))

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
// @Success 200 {object} serverResponse{data=convoy.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups/{groupID} [put]
func (a *applicationHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {

	var update models.Group
	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	groupName := update.Name
	if util.IsStringEmpty(groupName) {
		_ = render.Render(w, r, newErrorResponse("please provide a valid name", http.StatusBadRequest))
		return
	}

	group := getGroupFromContext(r.Context())
	group.Name = groupName
	group.Config = &update.Config
	err = a.groupRepo.UpdateGroup(r.Context(), group)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating Group", http.StatusInternalServerError))
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
// @Success 200 {object} serverResponse{data=[]convoy.Group}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /groups [get]
func (a *applicationHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	user := getAuthUserFromContext(r.Context())
	name := r.URL.Query().Get("name")
	userGroups := user.Role.Groups

	var filter *convoy.GroupFilter

	if !util.IsStringEmpty(name) {
		for _, g := range userGroups {
			if name == g {
				filter = &convoy.GroupFilter{Names: []string{name}}
				break
			}
		}

		if filter == nil {
			_ = render.Render(w, r, newErrorResponse("invalid group access", http.StatusForbidden))
			return
		}
	} else if user.Role.Type == auth.RoleSuperUser {
		filter = &convoy.GroupFilter{}
	} else {
		filter = &convoy.GroupFilter{Names: userGroups}
	}

	groups, err := a.groupRepo.LoadGroups(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching Groups", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Groups fetched successfully", groups, http.StatusOK))
}
