# syntax=docker/dockerfile:1
FROM node:14 as node-env
WORKDIR /app
COPY ./web/ui/react-app .
RUN npm install --production
RUN npm run build
#EXPOSE 3000
#CMD [ "npm", "run", "start" ]

FROM golang:1.16 as build-env
WORKDIR /go/src/frain-dev/hookcamp


# Create a netrc file using the credentials specified using --build-arg

# ARG ACCESS_TOKEN="nothing"
# ARG ACCESS_TOKEN_USR="nothing"

# RUN printf "machine github.com\n\
#     login ${ACCESS_TOKEN_USR}\n\
#     password ${ACCESS_TOKEN}\n\
#     \n\
#     machine api.github.com\n\
#     login ${ACCESS_TOKEN_USR}\n\
#     password ${ACCESS_TOKEN}\n"\
#     >> /root/.netrc
# RUN chmod 600 /root/.netrc

COPY ./go.mod /go/src/frain-dev/hookcamp
COPY ./go.sum /go/src/frain-dev/hookcamp
COPY ./hookcamp.json /go/src/frain-dev/hookcamp
COPY ./hookcamp-docker.json /go/src/frain-dev/hookcamp

COPY --from=node-env /app/build /go/src/frain-dev/hookcamp/server/ui/build
# Get dependancies - will also be cached if we don't change mod/sum
RUN go mod download
RUN go mod verify

# COPY the source code as the last step
COPY . .

RUN CGO_ENABLED=0
RUN go install ./cmd

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/cmd /
COPY --from=build-env /go/src/frain-dev/hookcamp/hookcamp.json /hookcamp.json
COPY --from=build-env /go/src/frain-dev/hookcamp/hookcamp-docker.json /hookcamp-docker.json

EXPOSE 5005
EXPOSE 27017
EXPOSE 27037
EXPOSE 6379
EXPOSE 8379
