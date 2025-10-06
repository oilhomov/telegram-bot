# builder stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git build-base ffmpeg yt-dlp

WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org,direct
RUN go mod download

COPY . .
RUN go build -o /bot main.go

# final stage
FROM alpine:3.18
RUN apk add --no-cache ffmpeg yt-dlp ca-certificates
WORKDIR /root/
COPY --from=builder /bot /root/bot
# create temp dir
RUN mkdir -p /tmp
CMD ["/root/bot"]
