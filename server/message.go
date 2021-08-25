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

			msg := &hookcamp.Message{
				UID:       uuid.New().String(),
				AppID:     app.UID,
				EventType: hookcamp.EventType(eventType),
				Data:      d,
				Metadata: &hookcamp.MessageMetadata{
					NumTrials:    0,
					RetryLimit:   10,
					NextSendTime: time.Now().Unix(),
				},
				MessageAttempts: make([]hookcamp.MessageAttempt, 0),
				CreatedAt:       time.Now().Unix(),
				UpdatedAt:       time.Now().Unix(),
				Application:     app,
				Status:          hookcamp.ScheduledMessageStatus,
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

func fetchAllMessages(msgRepo hookcamp.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			page := getPageFromContext(r.Context())
			perPage := getPageSizeFromContext(r.Context())
			pageable := models.Pageable{
				Page:    page,
				PerPage: perPage,
			}

			m, paginationData, err := msgRepo.LoadMessagesPaged(r.Context(), pageable)
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
