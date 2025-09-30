# Minify client side assets (JavaScript)
FROM node:18-alpine AS build-js

RUN npm install gulp gulp-cli -g

WORKDIR /build
COPY package*.json ./
RUN npm install --only=dev
COPY . .
RUN gulp

# Build Golang binary (Pure Go - no CGO needed for PostgreSQL)
FROM golang:1.24-alpine AS build-golang

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -v -o fyphish .

# Runtime container
FROM alpine:latest

RUN adduser -D -h /opt/fyphish -s /bin/sh app

RUN apk add --no-cache ca-certificates jq envsubst

WORKDIR /opt/fyphish
COPY --from=build-golang /app/fyphish ./
COPY --from=build-js /build/static/js/dist/ ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/
COPY --from=build-golang /app/static/ ./static/
COPY --from=build-golang /app/templates/ ./templates/
COPY --from=build-golang /app/db/ ./db/
COPY --from=build-golang /app/docker/run.sh ./
COPY --from=build-golang /app/VERSION ./
COPY config.json ./

RUN chown -R app:app . && chmod +x run.sh

USER app

EXPOSE 3333 8080 8443 80

CMD ["./run.sh"]