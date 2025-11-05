#!/usr/bin/env bash
set -euo pipefail
# Run blst-tagged benchmarks for bls381 and print simple baseline summary.
# Usage: ./deploy/testnet/tools/tss_bench.sh

go test -tags blst ./internal/tss/core/bls381 -bench . -benchmem -count 5 | tee tss_bench_baseline.txt