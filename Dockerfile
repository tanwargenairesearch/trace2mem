FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/brain-server ./cmd/brain-server && \
    CGO_ENABLED=0 go build -trimpath -o /out/brain-worker ./cmd/brain-worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/brainctl ./cmd/brainctl
FROM debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818 AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates fuse3 git && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m brain
COPY --from=build /out/ /usr/local/bin/
RUN mkdir -p /data/blobs /data/secrets && chown -R brain:brain /data
COPY scripts/fuse.sh /usr/local/bin/brain-fuse-test
USER brain
WORKDIR /data
ENTRYPOINT ["brain-server"]
FROM build AS test
CMD ["go","test","./..."]
