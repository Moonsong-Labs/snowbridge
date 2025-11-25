# Upstream Merge Progress Tracker

Merging commits from `upstream/main` into `solochain` branch.

- **Working branch**: `solochain-merge-upstream`
- **Source**: `upstream/main`
- **Target**: `solochain`
- **Merge base**: `f53df1f9` - Fix Mythos hanging Txs (#1476)
- **Total commits**: 117
- **Status**: COMPLETE

## Status Legend
- [x] Completed
- [!] Conflict resolved
- [S] Skipped (empty after resolution)

## Commits to Merge (oldest to newest)

| # | Status | Hash | Description |
|---|--------|------|-------------|
| 1 | [!] | d836eb3e | Fix breaking smoke tests (#1472) | *Kept solochain gateway.go* |
| 2 | [x] | 5808ed7d | Fix moonbeam asset registry (#1482) |
| 3 | [x] | 64e54b60 | Send Token to Kusama (#1463) |
| 4 | [x] | a09a2efd | Unban LDO (#1488) |
| 5 | [x] | 972448b4 | Kusama transfer history (#1484) |
| 6 | [!] | 876eed00 | Fix on-demand relaying of Beefy commitments (#1449) | *Kept solochain scanner.go* |
| 7 | [x] | 7dd864f5 | Add acala (#1485) |
| 8 | [x] | 2a4a1f8a | Update default fee in API (#1417) |
| 9 | [x] | b2edc7d4 | More refactoring (#1490) |
| 10 | [x] | 702b9d07 | Gas Check for Submit V1 (#1497) |
| 11 | [x] | 7f42d5e7 | Adds min balance to Kusama fee (#1498) |
| 12 | [x] | af9e2cf5 | Fix moonbeam balance fetches (#1499) |
| 13 | [x] | 8917ece5 | API Override metadata (#1500) |
| 14 | [x] | a1a2857b | Improve E2E tests (#1496) |
| 15 | [x] | fdf95106 | Remove MYTH token from Kusama assets (#1503) |
| 16 | [x] | 50cfe070 | Minor API tweaks and fixes (#1504) |
| 17 | [x] | ab2883b0 | update paseo contracts (#1502) |
| 18 | [x] | 4e9be74f | Check for isFinalized in case of re-orgs (#1506) |
| 19 | [x] | 010806b1 | Update GQL with a limit & Some cleanup (#1508) |
| 20 | [x] | 089e04cd | Support XRQCY from Frequency parachain (#1487) |
| 21 | [x] | 891c8324 | Fix query sync status from Subsquid on Westend (#1514) |
| 22 | [!] | 4ef1ab82 | Registry NPM package (#1511) | *Kept npm-publish.yml deleted* |
| 23 | [x] | fd1379b4 | Up version numbers (#1517) |
| 24 | [x] | 76f4b44d | Add Curio token to Kusama registrations (#1516) |
| 25 | [x] | 7d66d101 | Update Ethereum client fixture command (#1509) |
| 26 | [x] | c4fa9d6c | Fix version number (#1518) |
| 27 | [x] | 6cf0221b | Add TRAC and XRT (#1519) |
| 28 | [x] | 38fb139b | Add Slippage Pad Percentage for Swap Fees (#1520) |
| 29 | [x] | f0df9afa | Fix appendix instruction with native asset as fee (#1521) |
| 30 | [x] | 78bba702 | Supress to-polkadot-channel-stale alarms (#1522) |
| 31 | [x] | ea620f82 | Snowbridge V2: Outbound transfer API (#1492) |
| 32 | [x] | 0d5f2494 | API Updates (#1524) |
| 33 | [x] | e3534e92 | Simplify MakeTrie function and perform code cleanup (#1525) |
| 34 | [x] | fc304bbf | Upgrade to V2 contracts (#1523) |
| 35 | [!] | 69f37534 | Minor fix (#1526) | *Auto-resolved* |
| 36 | [x] | 1581e21e | GITBOOK-101: Developer docs |
| 37 | [x] | 4c3f7c25 | GITBOOK-103: change request with no subject merged in GitBook |
| 38 | [x] | 2313e170 | GITBOOK-104: change request with no subject merged in GitBook |
| 39 | [x] | c927c75c | Reorganize code structure by transfer types & Diversify fee options (#1528) |
| 40 | [x] | c8c2e939 | Update registry for frequency chains (#1531) |
| 41 | [x] | 07e3acbe | chore: fix function name in comment (#1512) |
| 42 | [x] | 13727b61 | GITBOOK-106: change request with no subject merged in GitBook |
| 43 | [x] | 15d28261 | Add local transfers (#1536) |
| 44 | [x] | 6dcec3c2 | Fix setup scripts (#1541) |
| 45 | [x] | ce74f152 | Improve alarms (#1538) |
| 46 | [x] | b5d871f6 | chore: fix some minor issues in comments (#1534) |
| 47 | [x] | 55cb577c | GITBOOK-107: change request with no subject merged in GitBook |
| 48 | [S] | e5ef9969 | GITBOOK-108: change request with no subject merged in GitBook | *Empty after resolution* |
| 49 | [x] | a0ed730a | GITBOOK-110: Parachain Integration Doc |
| 50 | [x] | 5a8184e5 | Fix the monitoring substrate address (#1543) |
| 51 | [x] | e7dde88e | Optimise fetching all receipts (#1547) |
| 52 | [x] | a5aef58e | bug bounty readme (#1548) |
| 53 | [x] | 2e470f99 | Add WUD to Kusama (#1550) |
| 54 | [x] | 3aba4b5f | chore: fix some typos in comment (#1555) |
| 55 | [!] | 16d3359f | Backport changes from V1 (#1559) | *Kept solochain main.go, updated heartbeat API* |
| 56 | [!] | 31c3a2c0 | Publish web packages from CI (#1535) | *Kept npm-publish.yml deleted* |
| 57 | [x] | 9fdd8ae3 | Deprecate polkadotXcm.transferAssets (#1558) |
| 58 | [x] | 4775f189 | Register ERC-20 metadata on Polkadot and Kusama (#1556) |
| 59 | [x] | 14ac83df | Adds NeuroWeb to Paseo (#1552) |
| 60 | [x] | 504f3541 | Improve V2 to Ethereum API (#1546) |
| 61 | [x] | 6d4062f5 | Monitor V2 (#1561) |
| 62 | [x] | 955a9711 | Upgrade Subxt and Forge (#1553) |
| 63 | [!] | bac8c954 | Relayer: Profit Estimation (#1510) | *Integrated with solochain naming* |
| 64 | [x] | e5229c13 | Use nix geth (#1563) |
| 65 | [x] | a9591694 | Neuroweb Fixes (#1564) |
| 66 | [x] | 30f3900e | Update README.md (#1567) |
| 67 | [x] | b4ada10e | Neuroweb mainnet config (#1570) |
| 68 | [x] | 4c74d403 | Revert "Use nix geth (#1563)" (#1569) |
| 69 | [x] | 3cfbf028 | Beefy client v2 (#1571) |
| 70 | [x] | 23345dbc | Update control tool (#1566) |
| 71 | [!] | 07e0ae32 | Limit calldata length (#1577) | *Kept both _verifyBeefyProof and transactionBaseGas* |
| 72 | [x] | 30ff3bec | Upgrade Polkadot dependencies (#1575) |
| 73 | [x] | 8662d767 | Ops scripts v2 (#1573) |
| 74 | [x] | 230f2052 | Detect equivocation and trigger an alarm (#1580) |
| 75 | [x] | 61968d9b | Support WUD transfers (#1578) |
| 76 | [x] | b51f9e83 | Control command for the V2 upgrade (#1587) |
| 77 | [x] | 8d581696 | Improve beefy relay (#1581) (#1585) |
| 78 | [x] | 4329251c | Replay failed XCM messages command (#1589) |
| 79 | [!] | 1b27a2c6 | Snowbridge V2 to Polkadot API (#1540) | *Kept solochain BeefyClient ABI (beefyExtraField)* |
| 80 | [x] | 71a4adfe | Support Neuroweb To Ethereum (#1593) |
| 81 | [x] | cb278ffc | Enforce dry run on BH (#1591) |
| 82 | [x] | e5b7ccf6 | GITBOOK-111: change request with no subject merged in GitBook |
| 83 | [x] | 286d685a | Fix local setup (#1594) |
| 84 | [x] | c2bcf196 | Remove limit on neuro (#1596) |
| 85 | [x] | 6a18966b | Fix Neuro asset filter (#1597) |
| 86 | [x] | 236d9f5c | More V1 tests on upgraded V2 Gateway (#1595) |
| 87 | [x] | f2b8dfb1 | Fulu (#1592) |
| 88 | [S] | 94351780 | chore: fix wrong struct field name in comment (#1586) | *Empty - kept solochain ByOutboundMessage* |
| 89 | [x] | 61aac2c7 | chore: fix some minor issues in the comments (#1582) |
| 90 | [!] | daaab800 | Fix cli command (#1600) | *Integrated private key file/ID for solochain* |
| 91 | [x] | 28fe59db | Set Paseo Snowbridge v2 fee (#1601) |
| 92 | [x] | 1fdd9321 | Switch beefy client (#1604) |
| 93 | [x] | 9b576669 | handle errors (#1588) |
| 94 | [!] | ee44d177 | Ethereum to Polkadot Gas Estimator (#1527) | *Added missing imports* |
| 95 | [x] | 1278c272 | Fix moonbeam balance fetch (#1607) |
| 96 | [x] | f4fe814e | Improve monitor with sync status from any parachain (#1602) |
| 97 | [x] | 2c0e3816 | More Beefy tests (#1603) |
| 98 | [!] | d710cd3a | Generate message proof off-chain (#1608) | *See note below* |
| 99 | [x] | 64f7e433 | update moonbeam url (#1610) |
| 100 | [x] | c9bde369 | Custom XCM Support Ethereum to Polkadot (#1613) |
| 101 | [x] | 14b3c8ce | GITBOOK-112: change request with no subject merged in GitBook |
| 102 | [x] | 31407269 | L2 by transact (#1609) |
| 103 | [x] | 186b378f | Beefy pipeline v2 (#1611) |
| 104 | [x] | 04ab064b | Format ts (#1618) |
| 105 | [x] | 187a445e | Gas estimator tweaks (#1616) |
| 106 | [x] | 4dcf5fe1 | update bindings (#1615) |
| 107 | [x] | 35cc0cb5 | Fix alarm config for Undelivered Timeout (#1612) |
| 108 | [x] | d02a7ce4 | Snowbridge V2 SDK Register Agent (#1620) |
| 109 | [x] | b3b2ede5 | docs: minor improvement for docs (#1624) |
| 110 | [x] | 4792ea6b | GITBOOK-113: Snowbridge V2 Create Agent |
| 111 | [x] | 5add5cde | Custom Ethereum Transact (#1617) |
| 112 | [x] | b77e0b4c | Suppress transient error (#1622) |
| 113 | [x] | a9e6955b | OFAC checks v2 (#1621) |
| 114 | [x] | 4564ac2a | Update import beacon state helper command (#1623) |
| 115 | [x] | b54c8eb6 | Update metadata And enable Dry Run on NeuroWeb (#1625) |
| 116 | [x] | 665fc224 | Snowbridge V2 SDK Register Tokens (#1619) |
| 117 | [!] | a684acc7 | Seperate the delivery-reward relay (#1629) | *Kept solochain components* |

## Solochain-Specific Resolution Notes

### Commit 98: Proof Generation Strategy (d710cd3a)

For solochain, we implemented a **two-tier fallback approach** for obtaining message proofs:

1. **PRIMARY**: Fetch proofs from chain storage via `scanForOutboundQueueProofs()`
   - Queries the solochain's EthereumOutboundQueueV2 pallet directly
   - Preferred because it uses the chain's own proof computation

2. **FALLBACK**: Compute proofs off-chain via `buildOutboundQueueProofs()`
   - Used if chain storage is unavailable (e.g., pruned state, RPC issues)
   - Fetches MessageLeaves and computes merkle tree locally

This differs from upstream parachain which only uses off-chain computation.
We maintain chain-based fetching as primary for solochain compatibility.

### Commit 117: Delivery-Reward Relay Separation (a684acc7)

Kept solochain-specific components that upstream removed:
- `solochainWriter` - ParachainWriter for solochain
- `beaconHeader` - Header sync component
- `headerCache` - Ethereum header cache
- `parachain-writer.go` - File kept (upstream deleted)
- Config uses `Solochain` instead of `Polkadot/Parachain`

## Next Steps
1. Build and test the merged code
2. Run relayer tests
3. Create PR to merge `solochain-merge-upstream` into `solochain`
