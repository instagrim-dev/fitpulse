# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /workspace
COPY services/activity-service/go.mod services/activity-service/go.sum ./services/activity-service/
COPY libs/go ./libs/go
WORKDIR /workspace/services/activity-service
RUN go mod download
COPY services/activity-service ./ 
RUN CGO_ENABLED=0 go build -trimpath -o /workspace/bin/activity ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -o /workspace/bin/activity-dlq-manager ./cmd/dlqmanager
RUN CGO_ENABLED=0 go build -trimpath -o /workspace/bin/activity-consumer ./cmd/consumer

FROM alpine:3.20 AS runtime
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /home/appuser
COPY --from=builder /workspace/bin/activity /usr/local/bin/activity
COPY --from=builder /workspace/bin/activity-dlq-manager /usr/local/bin/activity-dlq-manager
COPY --from=builder /workspace/bin/activity-consumer /usr/local/bin/activity-consumer
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/activity"]
