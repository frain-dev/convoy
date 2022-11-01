FROM alpine:3.16.2

COPY convoy /cmd
RUN chmod +x /cmd
RUN apk add --no-cache gcompat 
ENTRYPOINT [ "/cmd" ]
CMD ["server", "--config", "/convoy.json"] 