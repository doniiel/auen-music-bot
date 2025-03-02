FROM golang:1.20 AS builder

RUN apt-get update && apt-get install -y ffmpeg python3-pip
RUN pip3 install yt-dlp

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/music-bot ./cmd/bot

FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y ffmpeg python3-pip
RUN pip3 install yt-dlp

WORKDIR /app
COPY --from=builder /app/mybot /usr/local/bin/mybot

CMD ["/usr/local/bin/music-bot"]
