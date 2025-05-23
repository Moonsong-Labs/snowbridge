# Fork Info

This is a fork of the Snowbridge project (docs below).

## Branching Strategy

- `main` follows and should always match the `main` branch of the Snowbridge repo.
- `solochain` is the default branch, and the one with all the custom changes made for Snowbridge to work with a Substrate solochain instead of the BridgeHub parachain in Polkadot. PRs with new features and changes should be made to this branch.
- There's a [PR](https://github.com/Moonsong-Labs/snowbridge/pull/9) to track the diff between the `main` branch and the `solochain` branch.

# Snowbridge

[![codecov](https://codecov.io/gh/Snowfork/snowbridge/branch/main/graph/badge.svg?token=9hvgSws4rN)](https://codecov.io/gh/Snowfork/snowbridge)
![GitHub](https://img.shields.io/github/license/Snowfork/snowbridge)

Snowbridge is a trustless bridge between Polkadot and Ethereum. For documentation, visit https://docs.snowbridge.network.

## Components

The Snowbridge project lives in two repositories:

- [Snowfork/polkadot-sdk](https://github.com/Snowfork/polkadot-sdk): The Snowbridge parachain and pallets live in
  a fork of the polkadot-sdk. Changes are eventually contributed back to
  [paritytech/polkadot-sdk](https://github.com/paritytech/polkadot-sdk)
- [Snowfork/snowbridge](https://github.com/Snowfork/snowbridge): The rest of the Snowbridge components, like contracts,
  off-chain relayer, end-to-end tests and test-net setup code.

### Parachain

Polkadot parachain and our pallets. See [https://github.com/Snowfork/polkadot-sdk](https://github.com/Snowfork/polkadot-sdk/blob/snowbridge/bridges/snowbridge/README.md).

### Contracts

Ethereum contracts and unit tests. See [contracts/README.md](https://github.com/Snowfork/snowbridge/blob/main/contracts/README.md)

### Relayer

Off-chain relayer services for relaying messages between Polkadot and Ethereum. See
[relayer/README.md](https://github.com/Snowfork/snowbridge/blob/main/relayer/README.md)

### Local Testnet

Scripts to provision a local testnet, running the above services to bridge between local deployments of Polkadot and
Ethereum. See [web/packages/test/README.md](https://github.com/Snowfork/snowbridge/blob/main/web/packages/test/README.md).

### Smoke Tests

Integration tests for our local testnet. See [smoketest/README.md](https://github.com/Snowfork/snowbridge/blob/main/smoketest/README.md).

## Development

We use the Nix package manager to provide a reproducible and maintainable developer environment.

After [installing Nix](https://nixos.org/download.html), enable [flakes](https://wiki.nixos.org/wiki/Flakes):

```sh
mkdir -p ~/.config/nix
echo 'experimental-features = nix-command flakes' >> ~/.config/nix/nix.conf
```

Then activate a developer shell in the root of our repo, where
[`flake.nix`](https://github.com/Snowfork/snowbridge/blob/main/flake.nix) is located:

```sh
nix develop
```

Also make sure to run this initialization script once:

```sh
scripts/init.sh
```

### Support for code editors

To ensure your code editor (such as VS Code) can execute tools in the nix shell, startup your editor within the
interactive shell.

Example for VS Code:

```sh
nix develop
code .
```

### Custom shells

The developer shell is bash by default. To preserve your existing shell:

```sh
nix develop --command $SHELL
```

### Automatic developer shells

To automatically enter the developer shell whenever you open the project, install
[`direnv`](https://direnv.net/docs/installation.html) and use the template `.envrc`:

```sh
cp .envrc.example .envrc
direnv allow
```

### Upgrading the Rust toolchain

Sometimes we would like to upgrade rust toolchain. First update `polkadot-sdk/rust-toolchain.toml` as required and then
update `flake.lock` running

```sh
nix flake lock --update-input rust-overlay
```

## Troubleshooting

Check the contents of all `.envrc` files.

Remove untracked files:

```sh
git clean -idx
```

Ensure that the current Rust toolchain is the one selected in `scripts/init.sh`.

Ensure submodules are up-to-date:

```sh
git submodule update
```

Check untracked files & directories:

```sh
git clean -ndx | awk '{print $3}'
```

After removing `node_modules` directories (eg. with `git clean above`), clear the pnpm cache:

```sh
pnpm store prune
```

Check Nix config in `~/.config/nix/nix.conf`.

Run a pure developer shell (note that this removes access to your local tools):

```sh
nix develop -i --pure-eval
```

## Security

The security policy and procedures can be found in SECURITY.md.
