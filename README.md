Aequa DVT Sequencer

Language · 语言
- English | [简体中文](#简体中文)

---

# English

Introduction
- Aequa is a production‑minded, modular Distributed Validator (DVT) sequencer. The core includes API, P2P, QBFT consensus, and StateDB. Optional components (behind feature flags, default off) include TSS, a pluggable mempool, and a deterministic payload builder. Observability and CI/security gates are first‑class with a strict “zero label drift” policy.
- Out of scope for now (under active development): BEAST/MEV, DFBA. They will be integrated later behind flags without changing existing metric/log label sets.

Vision
- Deliver a reliable, observable, security‑gated DVT engine that is easy to gray‑roll out and roll back. Defaults are safe: experimental features are off; compatibility and metrics/logging stability are prioritized for long‑run operations and audits.

Architecture (Overview)
- API Service: JSON endpoints (`/health`, `/v1/duty`) with strict validation and unified JSON logs.
- Monitoring Service: Prometheus exposition `/metrics` with stable metric families and labels.
- P2P Service: AllowList → Rate → Score gates, resource manager, DKG/cluster‑lock fail‑fast checks, optional TSS session limiter.
- Consensus: QBFT verifier (strict + anti‑replay) and minimal state machine (preprepare/prepare/commit with dedup); WAL for vote intents.
- StateDB: atomic write + CRC + `.bak` fallback with pessimistic recovery.
- Optional (behind flags): mempool + builder (deterministic selection), header verification via TSS aggregate signatures, and signing path.
- E2E test endpoints: compiled only with `-tags e2e` (e.g., `/e2e/qbft` at :4610).

Core Metrics & Logs (stable)
- Metrics: `api_requests_total{route,code}`, `api_latency_ms_sum/_count{route}`, `service_op_ms_sum/_count{service,op}`, `consensus_events_total{kind}`, `consensus_proc_ms_sum/_count{kind}`, `qbft_msg_verified_total{result|type}`, `qbft_state_transitions_total{type}`, `qbft_qc_built_total{kind}`, `p2p_conn_attempts_total{result}`, `p2p_conns_open`, `p2p_conn_open_total/close_total`, `state_persist_ms_sum/_count`, `state_recovery_total{result}`
- Logs: unified JSON with `trace_id, route, code, result, latency_ms, err?` and component‑specific context

Invariants (Iron Rules)
- Features are behind flags/tags and off by default; do not change existing metric/log labels.
- Zero dimension drift: only add new metric families; do not alter existing label sets or names.
- UTF‑8 without BOM enforced by CI (blocking).
- Coverage: `internal/api` ≥70%; `internal/tss/*` (final target ≥70%); fuzz has no panics.
- Small PRs + revertability; dependency whitelist + version pinning; `govulncheck`/Snyk green.

QBFT “Voting” Flow
- MsgPreprepare: leader proposes (constraint: round must be 0 in current code).
- MsgPrepare: nodes collect Prepare after Preprepare; when prepares ≥2 (placeholder threshold), enter prepared.
- MsgCommit: after prepared, collect Commit; current placeholder threshold is ≥1 to enter commit (increments `qbft_qc_built_total{kind="commit"}`).
- Verifier (BasicVerifier): strict structure/type checks, round/height windows, anti‑replay (ID or height‑window), signature‑shape placeholder. Logs results; increments `qbft_msg_verified_total{result|type}`.

How To Test Voting (e2e + adversary‑agent)
0) Build with e2e tag: `docker build --build-arg BUILD_TAGS=e2e -t aequa-local:latest .`
1) Baseline ops (no voting injection): `docker compose -f deploy/testnet/docker-compose.yml up -d`; check health/metrics and import `deploy/testnet/grafana/dashboard.json`; try `rolling_upgrade.sh`/`rollback_all.sh`.
2) Voting & resilience: `docker compose -f docker-compose.yml up -d`; the `adversary-agent` sends voting messages to `:4610/e2e/qbft` (all nodes). Observe panels “QBFT state transitions by type” and `qbft_qc_built_total{kind=prepare|commit}`. For chaos: `./.github/scripts/netem.sh apply partition` then `clear`, or restart a node; rates should drop and recover accordingly.
3) Optional TSS microbench: `bash ./deploy/testnet/tools/tss_bench.sh` or `go test -tags blst ./internal/tss/core/bls381 -bench . -benchmem -count 5`.

Deployment
- Native binary: `go build -o bin/dvt-node ./cmd/dvt-node` then run `./bin/dvt-node --validator-api 0.0.0.0:4600 --monitoring 0.0.0.0:4620`
- Docker (4‑node minimal): `docker build -t aequa-local:latest . && docker compose -f deploy/testnet/docker-compose.yml up -d`
- Grafana: import `deploy/testnet/grafana/dashboard.json`; alert suggestions in `deploy/testnet/grafana/alerts.md`

Internal Testnet Plan (Phased)
- Phase 0: Build e2e image
- Phase 1: Baseline ops (no attacker)
- Phase 2: Voting + chaos (attacker + netem)
- Phase 3: Optional TSS benchmarks

CI / Security / Compliance
- CI: verify‑bom, dependency whitelist (`scripts/dep-whitelist.sh`), golangci‑lint, unit tests with coverage gate (`internal/api`), `govulncheck`, Snyk, qbft tests.
- Compliance: see `deploy/testnet/compliance/WHITELIST.md`, `deploy/testnet/compliance/LICENSES.md`.
- TSS deps: `blst` only via build tag; pin versions; separate CI job if enabling tag‑build.

Roadmap
- TSS GA (per PLAN_v2 Appendix); robust DKG retry/complaint loops; performance baselines; gray rollout; long‑run stability.

License
- Business Source License 1.1 (BSL 1.1). See `LICENSE`.

---

# 简体中文

简介
- Aequa 是一个面向生产的模块化分布式验证器（DVT）定序引擎。核心包含 API、P2P、QBFT 共识与 StateDB；可选组件（behind‑flag，默认关闭）包括 TSS、可插拔内存池与确定性 Payload Builder。项目以可观测性与 CI/安全门禁为基础，严格执行“维度零漂移”。
- 暂不包含（开发中）：BEAST/MEV、DFBA。未来将以 behind‑flag 方式接入，不改变既有指标/日志标签集与名称。

愿景
- 提供可靠、可观测、可审计、易灰度与可回滚的 DVT 定序引擎；默认安全（实验特性默认关闭）、与既有生态兼容；支撑长测与审计。

架构概览
- API：提供 `/health`、`/v1/duty` 等接口，统一 JSON 日志与严格校验。
- 监控：`/metrics` 导出稳定的 Prometheus 指标族与标签。
- P2P：AllowList → Rate → Score 门禁管线、资源管理器、DKG/集群锁 fail‑fast、自适应 TSS 会话上限（可选）。
- 共识：QBFT 验证器（严格 + 反重放）与简化状态机（preprepare/prepare/commit 去重）；投票意图 WAL。
- 状态存储：原子写入 + CRC + `.bak` 回退，保守恢复策略。
- 可选：mempool + builder（确定性选择）、基于 TSS 的头验证与签名路径（behind‑flag）。
- E2E 端点：仅在 `-tags e2e` 编译时启用（如 `:4610/e2e/qbft`）。

核心指标/日志（稳定）
- 指标：`api_requests_total{route,code}`、`api_latency_ms_sum/_count{route}`、`service_op_ms_sum/_count{service,op}`、`consensus_events_total{kind}`、`consensus_proc_ms_sum/_count{kind}`、`qbft_msg_verified_total{result|type}`、`qbft_state_transitions_total{type}`、`qbft_qc_built_total{kind}`、`p2p_conn_attempts_total{result}`、`p2p_conns_open`、`p2p_conn_open_total/close_total`、`state_persist_ms_sum/_count`、`state_recovery_total{result}` 等。
- 日志：统一 JSON 字段 `trace_id, route, code, result, latency_ms, err?` 与上下文字段。

重要守则（铁律）
- 默认关闭：任何新功能 behind flag/tag，默认 off；不改既有指标/日志 label。
- 维度零漂移：只能新增 family，不可修改既有 label 集与名称。
- BOM 零容忍：UTF‑8 无 BOM；CI `verify-bom` 阻断。
- 覆盖率：`internal/api` ≥70%；`internal/tss/*` 最终统一 ≥70%；Fuzz 无 panic。
- 小步 PR + 可回滚；依赖白名单/版本固定；`govulncheck`/Snyk 绿灯。

QBFT “投票”流程
- MsgPreprepare：提议节点（Leader）发起提议（当前代码约束：round 必须为 0）。
- MsgPrepare：收到 Preprepare 后，收集 Prepare；当 Prepare 票数 ≥2（占位阈值）进入 prepared 状态。
- MsgCommit：在 prepared 后收集 Commit；当前以 ≥1（占位阈值）进入 commit（会递增 `qbft_qc_built_total{kind="commit"}`）。
- 验证层（BasicVerifier）：结构与类型校验、轮次/高度窗口、反重放（ID 或高度窗口）、签名“形状”占位；统一记录 `qbft_msg_verified_total{result|type}`。

如何测试“投票”（e2e + adversary‑agent）
第 0 阶段：准备（务必使用 e2e 构建标签）
- 构建镜像：`docker build --build-arg BUILD_TAGS=e2e -t aequa-local:latest .`

第 1 阶段：基础功能与运维（无“投票”注入）
- 启动 4 节点最小集群：`docker compose -f deploy/testnet/docker-compose.yml up -d`
- 健康/指标：`curl 127.0.0.1:4600/health`；`curl 127.0.0.1:4620/metrics`
- 仪表盘：导入 `deploy/testnet/grafana/dashboard.json`
- 运维脚本：`deploy/testnet/tools/rolling_upgrade.sh`、`rollback_all.sh`

第 2 阶段：核心共识（投票）与弹性测试
- 启动完整集群（含 adversary‑agent 与 netem）：`docker compose -f docker-compose.yml up -d`
- 验证成功性：面板“QBFT state transitions by type”应看到 preprepare/prepare/commit 大于 0；指标 `qbft_qc_built_total{kind="prepare|commit"}` 随时间增长。
- 验证安全性：无效投票（重放、错误轮次、ID 不匹配等）应被拒绝；面板“QBFT verify results”中的 `replay/error/unauthorized/round_oob` 应增长。
- 混沌/对抗：`./.github/scripts/netem.sh apply partition`（100% 丢包）→ `clear` 恢复；或 `docker compose restart aequa-node-1` 模拟 churn；预期投票速率下降后可恢复。

第 3 阶段（可选）：TSS 性能基准
- 脚本：`bash ./deploy/testnet/tools/tss_bench.sh`
- 原生命令：`go test -tags blst ./internal/tss/core/bls381 -bench . -benchmem -count 5`

部署
- 原生：`go build -o bin/dvt-node ./cmd/dvt-node` → `./bin/dvt-node --validator-api 0.0.0.0:4600 --monitoring 0.0.0.0:4620`
- Docker（最小 4 节点）：`docker build -t aequa-local:latest . && docker compose -f deploy/testnet/docker-compose.yml up -d`
- Grafana：导入 `deploy/testnet/grafana/dashboard.json`；告警建议见 `deploy/testnet/grafana/alerts.md`

内部测试网计划（分阶段）
- 阶段 0：e2e 镜像构建
- 阶段 1：基础运维（无 attacker）
- 阶段 2：投票 + 混沌（attacker + netem）
- 阶段 3：TSS 基准（可选）

CI / 合规 / 安全
- CI：verify‑bom、依赖白名单（`scripts/dep-whitelist.sh`）、golangci‑lint、单测覆盖率（`internal/api`）、`govulncheck`、Snyk、qbft tests。
- 合规：见 `deploy/testnet/compliance/WHITELIST.md`、`deploy/testnet/compliance/LICENSES.md`。
- TSS 依赖：`blst` 仅在 tag 构建启用；需版本固定；如需在 CI 跑 tag‑build，请添加专门 job。

路线图
- 按 `docs/PLAN_v2_TSS_GA_APPENDIX.md` 推进 TSS GA；完善 DKG 投诉/重试闭环；基线性能与 SLO；灰度上线；长测稳定性。

许可协议
- Business Source License 1.1 (BSL 1.1)，见 `LICENSE`。

