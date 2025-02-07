FROM alpine:3.20.2

# Define a build-time argument
ARG IMAGE_SHA

# Set an environment variable using the ARG
ENV CORE_GATEWAY_IMAGE_SHA=${IMAGE_SHA}

# Copy the Convoy binary
COPY convoy /cmd

# Copy the migrations directory
COPY sql/ /sql/

# Copy the startup script
COPY configs/local/start.sh /start.sh

# Set permissions
RUN chmod +x /cmd /start.sh

# Install necessary dependencies
RUN apk add --no-cache gcompat

# Set the startup command
CMD ["/start.sh"]

