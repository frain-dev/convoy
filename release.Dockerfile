FROM alpine:3.16.2

COPY convoy /convoy
RUN chmod +x /convoy
RUN apk add --no-cache gcompat 

CMD ["/convoy", "server", "--config", "/convoy.json"] 