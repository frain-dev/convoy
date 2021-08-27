package server

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/server/models"
	"github.com/hookcamp/hookcamp/util"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

func ensureNewMessage(appRepo hookcamp.ApplicationRepository, msgRepo hookcamp.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var newMessage models.Message
			err := json.NewDecoder(r.Body).Decode(&newMessage)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
				return
			}

			eventType := newMessage.EventType
			if util.IsStringEmpty(eventType) {
				_ = render.Render(w, r, newErrorResponse("please provide an eventType", http.StatusBadRequest))
				return
			}
			d := newMessage.Data
			if d == nil {
				_ = render.Render(w, r, newErrorResponse("please provide your data", http.StatusBadRequest))
				return
			}

			appID := chi.URLParam(r, "appID")
			if util.IsStringEmpty(appID) {
				_ = render.Render(w, r, newErrorResponse("please provide your appID", http.StatusBadRequest))
				return
			}
			app, err := appRepo.FindApplicationByID(r.Context(), appID)
			if err != nil {

				msg := "an error occurred while retrieving app details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrApplicationNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				log.Debugln("error while fetching app - ", err)

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}
			if len(app.Endpoints) == 0 {
				_ = render.Render(w, r, newErrorResponse("app has no configured endpoints", http.StatusBadRequest))
				return
			}

			msg := &hookcamp.Message{
				UID:       uuid.New().String(),
				AppID:     app.UID,
				EventType: hookcamp.EventType(eventType),
				Data:      d,
				Metadata: &hookcamp.MessageMetadata{
					NumTrials:    0,
					RetryLimit:   2,
					NextSendTime: primitive.NewDateTimeFromTime(time.Now()),
				},
				MessageAttempts: make([]hookcamp.MessageAttempt, 0),
				CreatedAt:       primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:       primitive.NewDateTimeFromTime(time.Now()),
				AppMetadata: &hookcamp.AppMetadata{
					OrgID:     app.OrgID,
					Endpoints: parseMetadataFromEndpoints(app.Endpoints),
				},
				Status: hookcamp.ScheduledMessageStatus,
			}

			err = msgRepo.CreateMessage(r.Context(), msg)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while creating message", http.StatusInternalServerError))
				return
			}
			r = r.WithContext(setMessageInContext(r.Context(), msg))
			next.ServeHTTP(w, r)
		})
	}
}

func parseMetadataFromEndpoints(endpoints []hookcamp.Endpoint) []hookcamp.EndpointMetadata {
	m := make([]hookcamp.EndpointMetadata, 0)
	for _, e := range endpoints {
		m = append(m, hookcamp.EndpointMetadata{
			UID:       e.UID,
			TargetURL: e.TargetURL,
			Merged:    false,
		})
	}
	return m
}

func fetchAllMessages(msgRepo hookcamp.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			pageable := getPageableFromContext(r.Context())

			orgId := r.URL.Query().Get("orgId")

			m, paginationData, err := msgRepo.LoadMessagesPaged(r.Context(), orgId, pageable)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching app messages", http.StatusInternalServerError))
				log.Errorln("error while fetching messages - ", err)
				return
			}

			r = r.WithContext(setMessagesInContext(r.Context(), &m))
			r = r.WithContext(setPaginationDataInContext(r.Context(), &paginationData))
			next.ServeHTTP(w, r)
		})
	}
}

func fetchAppMessages(appRepo hookcamp.ApplicationRepository, msgRepo hookcamp.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			appID := chi.URLParam(r, "appID")
			app, err := appRepo.FindApplicationByID(r.Context(), appID)
			if err != nil {

				msg := "an error occurred while retrieving app details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrApplicationNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				log.Errorln("error while fetching app - ", err)

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			m, err := msgRepo.LoadMessagesByAppId(r.Context(), app.UID)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching app messages", http.StatusInternalServerError))
				log.Errorln("error while fetching messages - ", err)
				return
			}

			r = r.WithContext(setMessagesInContext(r.Context(), &m))
			next.ServeHTTP(w, r)
		})
	}
}

func requireMessage(msgRepo hookcamp.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			msgId := chi.URLParam(r, "msgID")

			msg, err := msgRepo.FindMessageByID(r.Context(), msgId)
			if err != nil {

				msg := "an error occurred while retrieving message details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrMessageNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			r = r.WithContext(setMessageInContext(r.Context(), msg))
			next.ServeHTTP(w, r)
		})
	}
}
