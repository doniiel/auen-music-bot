# Этап сборки
FROM golang:1.23-alpine AS builder

# Устанавливаем необходимые пакеты: ffmpeg, python3, pip и git
RUN apk update && apk add --no-cache ffmpeg python3 py3-pip git

# Устанавливаем yt-dlp через pip с флагом --break-system-packages (на этапе сборки он работает)
RUN pip3 install --no-cache-dir --break-system-packages yt-dlp

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Собираем бинарник с отключенным cgo для статической сборки
RUN CGO_ENABLED=0 GOOS=linux go build -o music-bot ./cmd/bot

# Финальный образ: используем стабильную версию Alpine
FROM alpine:3.17

# Устанавливаем зависимости для рантайма
RUN apk add --no-cache ffmpeg python3 py3-pip ca-certificates && \
    pip3 install --no-cache-dir yt-dlp && \
    update-ca-certificates

WORKDIR /app
# Копируем собранный бинарник из этапа сборки
COPY --from=builder /app/music-bot /usr/local/bin/music-bot
COPY asset/banner.jpeg /app/banner.jpeg
COPY asset/logo.png /app/logo.png

CMD ["/usr/local/bin/music-bot"]
