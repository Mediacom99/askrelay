# askrelay relay image (WP-04 stub). Static binary, minimal runtime.
# Build:  docker build -f docs/deploy/Dockerfile -t askrelay .
# Run:    docker run -p 8080:8080 -v askrelay-data:/data \
#           -e ASKRELAY_BASE_URL=https://relay.example.com \
#           -e ASKRELAY_DB=/data/askrelay.db \
#           -e ASKRELAY_LISTEN=0.0.0.0:8080 askrelay
FROM golang:1.26 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /askrelay ./cmd/askrelay

FROM gcr.io/distroless/static-debian12
COPY --from=build /askrelay /askrelay
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/askrelay"]
CMD ["serve"]
