FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/trace2mem-server ./cmd/trace2mem-server && \
    CGO_ENABLED=0 go build -trimpath -o /out/trace2mem-worker ./cmd/trace2mem-worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/trace2mem ./cmd/trace2mem
FROM debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818 AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates fuse3 git && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m trace2mem
COPY --from=build /out/ /usr/local/bin/
RUN mkdir -p /data/blobs /data/secrets && chown -R trace2mem:trace2mem /data
COPY scripts/fuse.sh /usr/local/bin/trace2mem-fuse-test
USER trace2mem
WORKDIR /data
ENTRYPOINT ["trace2mem-server"]
FROM build AS test
CMD ["go","test","./..."]
