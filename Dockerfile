FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Сборка статичного бинарника
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/task-service \
    cmd/main.go

# Финальный минималистичный образ
FROM alpine:3.20 AS final

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Europe/Moscow

RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

# Скопировать бинарник и дефолтный конфиг
COPY --from=builder /app/task-service /app/task-service
COPY --from=builder /app/docs /app/docs
COPY config.yaml.example /app/config.yaml

# Выставляем права после скопированных файлов
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["./task-service"]