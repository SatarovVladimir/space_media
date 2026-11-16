FROM golang:1.27.3-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/order-worker ./cmd/order-worker

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app
RUN mkdir -p /data/uploads/avatars && chown -R app:app /data/uploads
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/order-worker /usr/local/bin/order-worker
USER app

EXPOSE 8080
CMD ["/usr/local/bin/api"]
