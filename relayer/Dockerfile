FROM golang:1.23
WORKDIR /opt/relayer
ADD . .
RUN go build -v -o build/snowbridge-relay main.go

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=0 /opt/relayer/build/snowbridge-relay /usr/local/bin/
VOLUME ["/config"]
ENTRYPOINT ["/usr/local/bin/snowbridge-relay"]
