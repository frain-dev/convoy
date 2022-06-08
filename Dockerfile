# syntax=docker/dockerfile:1
FROM node:14 as node-env
WORKDIR /app
COPY ./web/ui/dashboard .
RUN git config --global url."https://".insteadOf git://
RUN npm install
RUN npm run build

FROM golang:1.16 as build-env
WORKDIR /go/src/frain-dev/convoy

COPY ./go.mod /go/src/frain-dev/convoy
COPY ./go.sum /go/src/frain-dev/convoy

COPY --from=node-env /app/dist /go/src/frain-dev/convoy/server/ui/build
# Get dependancies - will also be cached if we don't change mod/sum
RUN go mod download
RUN go mod verify

# COPY the source code as the last step
COPY . .

RUN CGO_ENABLED=0
RUN go install ./cmd

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/cmd /
COPY --from=build-env /go/src/frain-dev/convoy/internal/email/templates/* templates/

ENTRYPOINT ["/cmd"]
CMD [ "server", "--config", "convoy.json" ]

EXPOSE 8080
