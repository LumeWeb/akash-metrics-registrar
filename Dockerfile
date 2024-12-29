# Use a minimal alpine image
FROM alpine:latest

# Install necessary system dependencies
RUN apk --no-cache add ca-certificates

# Copy the built executable
COPY ./akash-metrics-registrar /usr/bin/metrics-registrar

# Command to run the executable
ENTRYPOINT ["akash-metrics-registrar"]
