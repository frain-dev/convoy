package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/render"
	"github.com/google/uuid" 
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	err := a.fillGroupStatistics(r.Context(), group)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("failed to fetch group statistics", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Group fetched successfully",
		group, http.StatusOK))
}

func (a *applicationHandler) fillGroupStatistics(ctx context.Context, group *datastore.Group) error {
	appCount, err := a.appRepo.CountGroupApplications(ctx, group.UID)
	if err != nil {
		return fmt.Errorf("failed to count group messages: %v", err)
	}

	msgCount, err := a.eventRepo.CountGroupMessages(ctx, group.UID)
	if err != nil {
		return fmt.Errorf("failed to count group messages: %v", err)
	}

	group.Statistics = &datastore.GroupStatistics{
		MessagesSent: msgCount,
		TotalApps:    appCount,
	}

	return nil
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

	err := a.groupRepo.DeleteGroup(r.Context(), group.UID)
	if err != nil {
		log.WithError(err).Error("failed to delete group")
		_ = render.Render(w, r, newErrorResponse("failed to delete group", http.StatusInternalServerError))
		return
	}

	// TODO(daniel,subomi): is returning http error necessary for these? since the group itself has been deleted
	err = a.appRepo.DeleteGroupApps(r.Context(), group.UID)
	if err != nil {
		log.WithError(err).Error("failed to delete group apps")
		_ = render.Render(w, r, newErrorResponse("failed to delete group apps", http.StatusInternalServerError))
		return
	}

	err = a.eventRepo.DeleteGroupEvents(r.Context(), group.UID)
	if err != nil {
		log.WithError(err).Error("failed to delete group events")
		_ = render.Render(w, r, newErrorResponse("failed to delete group events", http.StatusInternalServerError))
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

	groupName := newGroup.Name
	if err = util.Validate(newGroup); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := &datastore.Group{
		UID:            uuid.New().String(),
		Name:           groupName,
		Config:         &newGroup.Config,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
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

	groupName := update.Name
	if err = util.Validate(update); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
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

	groups, err := a.groupRepo.LoadGroups(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching Groups", http.StatusInternalServerError))
		return
	}

	for _, group := range groups {
		err = a.fillGroupStatistics(r.Context(), group)
		if err != nil {
			log.WithError(err).Errorf("failed to fill statistics of group %s", group.UID)
		}
	}

	_ = render.Render(w, r, newServerResponse("Groups fetched successfully", groups, http.StatusOK))
}
