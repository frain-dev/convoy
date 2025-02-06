FROM alpine:3.20.2

# Define a build-time argument
ARG IMAGE_SHA

# Set an environment variable using the ARG
ENV CORE_GATEWAY_IMAGE_SHA=${IMAGE_SHA}

COPY convoy /cmd
COPY configs/local/start.sh /start.sh
COPY sql/* /sql
RUN chmod +x /cmd
RUN apk add --no-cache gcompat
CMD ["/start.sh"]
