FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /trmnl-wthr-svr .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder --chown=nonroot:nonroot /trmnl-wthr-svr /trmnl-wthr-svr

USER nonroot:nonroot

ENTRYPOINT ["/trmnl-wthr-svr", "server"]
