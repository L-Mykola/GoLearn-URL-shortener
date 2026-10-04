FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shortener ./cmd/shortener \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/shortener /shortener
COPY --from=build --chown=nonroot:nonroot /out/data /data

ENV ADDR=:8090 \
    DB_PATH=/data/shortener.db

EXPOSE 8090
VOLUME ["/data"]
USER nonroot:nonroot

ENTRYPOINT ["/shortener"]
