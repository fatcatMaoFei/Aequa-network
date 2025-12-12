Aequa Plan (M4 -> M5)
Last updated: 2025-12-11

Guiding principles
- Small incremental PRs, easy rollback; keep features behind flags (default off).
- No metric/log label drift; only add new families.
- Observability and tests first for every workstream.

Milestones
- M4 baseline (done): DVT sequencer core (API/P2P/QBFT/monitoring), plaintext_v1 mempool, e2e/chaos scripts.
- M5 value-capture readiness (in progress): DFBA ordering, MEV value accounting and sink interface, BEAST/privacy hooks staged.

Workstreams (with DFBA / BEAST landing path)
1) Tx and mempool extension
   - Add tx schema fields: feeRecipient, priorityFee/bid, payloadType (plaintext/auction/private).
   - Extend typed mempool: add auction_bid_v1 (bid-desc, nonce-aware), private_v1 (stub initially).
   - API validation and P2P gossip support for new types (topic compatible).
2) Deterministic builder / DFBA
   - Define BuilderPolicy (type order, MaxN), default covering auction_bid_v1 -> plaintext_v1.
   - Implement DFBA selection (bid/priority fee) with fixed window; inject policy at consensus start.
   - Observability: builder_selected_total by type; DFBA selection latency/result; rejection reasons.
   - Default params (can tune per-network, config-driven): order [auction_bid_v1, plaintext_v1], MaxN=1024, Window=MaxN, MinBid/MinFee=0, BatchTicks=0 (no time windowing by default).
3) Block value accounting and distribution outlet
   - Extend StandardBlock header with fee/bid aggregates.
   - Consensus commit hook emits {height, bids, priorityFees} accounting record.
   - Leave external settlement hook (fee sink/relayer) toward MEVDistributor contract, stub by default.
4) BEAST privacy pipeline (staged)
   - Swap TSS stub for blst build (tagged), expose Encrypt/Decrypt/VerifyAgg interfaces.
   - Add private_v1 path: API + P2P topic + builder decrypt-after-batch ordering (flagged).
   - Keep behind flag, default off; pending/privacy metrics.
   - Dev-only helper: enable JSON decrypt of private_v1 ciphertext via `-beast.json` flag (for testing without crypto).
5) SDK and DevEx
   - TS/Go client wrapping /v1/duty and /v1/tx/*; provide BEAST encrypt helper (stub ok initially).
   - Docs/examples: ethers.js/viem snippets.
6) Testing and ops
   - Unit tests for new paths; minimal DFBA/fee sink e2e.
   - Reliability: extend chaos scripts to new topics; rollback by disabling flags.

Next small PRs (deliverable-oriented)
- PR1: Inject default BuilderPolicy (plaintext_v1) and enable builder flag functionality; add metrics. (done locally)
- PR2: Extend tx schema + API validation + wire structs; add auction_bid_v1 typed mempool stub; set default builder order auction_bid_v1->plaintext_v1.
- PR3: Emit block value accounting records at commit (log/metric) and leave fee sink hook for MEVDistributor.
- PR4: Add BEAST stubs: private_v1 payload + API/P2P topic; TSS interface placeholders; flag-gated.
- PR5: Add SDK scaffold (TS) wrapping existing APIs; no onchain calls yet.
