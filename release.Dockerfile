FROM alpine:3.23.3

# Define a build-time argument
ARG IMAGE_SHA

# Set an environment variable using the ARG
ENV CORE_GATEWAY_IMAGE_SHA=${IMAGE_SHA}

# Copy the Convoy binary
COPY convoy /cmd

# Copy the migrations directory
COPY sql/ /sql/

# Set permissions
RUN chmod +x /cmd

# Install necessary dependencies
RUN apk add --no-cache gcompat

# Set the startup command
ENTRYPOINT ["/cmd"]
CMD ["server", "--config", "convoy.json"]

