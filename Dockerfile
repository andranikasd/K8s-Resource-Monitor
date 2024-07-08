# Stage 1: Build the Go binary
FROM golang:1.18 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o sidecar-watcher .

# Stage 2: Create the final image
FROM gcr.io/distroless/base-debian10

WORKDIR /root/

COPY --from=builder /app/sidecar-watcher .

EXPOSE 8080

CMD ["./sidecar-watcher"]
