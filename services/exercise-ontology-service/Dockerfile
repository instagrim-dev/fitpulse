# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /workspace
COPY services/exercise-ontology-service/go.mod services/exercise-ontology-service/go.sum ./services/exercise-ontology-service/
COPY libs/go ./libs/go
WORKDIR /workspace/services/exercise-ontology-service
RUN go mod download
COPY services/exercise-ontology-service ./
RUN CGO_ENABLED=0 go build -trimpath -o /workspace/bin/exercise-ontology ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -o /workspace/bin/exercise-ontology-consumer ./cmd/consumer

FROM alpine:3.20 AS runtime
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /home/appuser
COPY --from=builder /workspace/bin/exercise-ontology /usr/local/bin/exercise-ontology
COPY --from=builder /workspace/bin/exercise-ontology-consumer /usr/local/bin/exercise-ontology-consumer
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/exercise-ontology"]
