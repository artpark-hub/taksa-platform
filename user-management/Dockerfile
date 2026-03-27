# -------- Build stage --------
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache make git protobuf

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make init && make all && go build -o user-management ./cmd/user-management

# -------- Runtime stage --------
FROM alpine:3.23

RUN apk add --no-cache gettext

WORKDIR /app

COPY --from=builder /app/user-management .
COPY --from=builder /app/configs ./configs

EXPOSE 8083

CMD ["sh", "-c", "envsubst < ./configs/config.yaml > ./configs/config_resolved.yaml && ./user-management -conf ./configs/config_resolved.yaml"]
