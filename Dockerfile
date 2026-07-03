FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/migrate ./cmd/migrate

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate

EXPOSE 8080

USER nobody

CMD ["/bin/server"]
