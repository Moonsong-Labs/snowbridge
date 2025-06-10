FROM ghcr.io/foundry-rs/foundry AS contracts
WORKDIR /opt/contracts
# Copy specific files and directories, excluding lib
COPY contracts/src ./src
COPY contracts/test ./test
COPY contracts/scripts ./scripts
COPY contracts/foundry.toml ./foundry.toml
COPY contracts/remappings.txt ./remappings.txt

# Install dependencies fresh
RUN forge install --no-git foundry-rs/forge-std OpenZeppelin/openzeppelin-contracts dapphub/ds-test Snowfork/canonical-weth PaulRBerg/prb-math

# Clean and build
RUN forge clean && forge build

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