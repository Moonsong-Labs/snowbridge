FROM ghcr.io/foundry-rs/foundry AS contracts
WORKDIR /opt/contracts
COPY contracts .
RUN forge build

FROM golang:1.23 AS builder
WORKDIR /opt/relayer
COPY relayer .
COPY --from=contracts /opt/contracts/out /opt/contracts/out

RUN apt-get update && apt-get install -y jq
RUN go install github.com/ethereum/go-ethereum/cmd/abigen@latest
RUN go install github.com/ferranbt/fastssz/sszgen

RUN sszgen --path relays/beacon/state --objs BlockRootsContainerMainnet,TransactionsRootContainer,WithdrawalsRootContainerMainnet,BeaconStateDenebMainnet,BeaconBlockDenebMainnet,SignedBeaconBlockDeneb,SignedBeaconBlockElectra,BeaconStateElectra,BeaconBlockElectra
RUN go generate ./...
RUN go build -v -o build/snowbridge-relay main.go


FROM ubuntu:22.04
COPY --from=builder /opt/relayer/build/snowbridge-relay /usr/local/bin/
VOLUME ["/config"]
ENTRYPOINT ["/usr/local/bin/snowbridge-relay"]
