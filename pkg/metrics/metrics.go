package metrics

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type counterKey struct {
	name   string
	labels string
}

var (
	countersMu sync.RWMutex
	counters   = map[counterKey]*uint64{}
)

func labelsKey(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
	}
	return b.String()
}

func Inc(name string, labels map[string]string) {
	key := counterKey{name: name, labels: labelsKey(labels)}
	countersMu.RLock()
	p := counters[key]
	countersMu.RUnlock()
	if p == nil {
		countersMu.Lock()
		if counters[key] == nil {
			var v uint64
			counters[key] = &v
		}
		p = counters[key]
		countersMu.Unlock()
	}
	atomic.AddUint64(p, 1)
}

func ObserveSummary(name string, labels map[string]string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	key := summaryKey{name: name, labels: labelsKey(labels)}
	summaryMu.Lock()
	v := summaries[key]
	if v == nil {
		v = &summaryVal{}
		summaries[key] = v
	}
	v.sum += uint64(value)
	v.count++
	summaryMu.Unlock()
}

func DumpProm() string {
	countersMu.RLock()
	var sb strings.Builder
	sb.WriteString("# HELP dvt_up 1\n# TYPE dvt_up gauge\ndvt_up 1\n")
	keys := make([]counterKey, 0, len(counters))
	for k := range counters {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].labels < keys[j].labels
	})
	for _, k := range keys {
		v := atomic.LoadUint64(counters[k])
		if k.labels == "" {
			fmt.Fprintf(&sb, "%s %d\n", k.name, v)
		} else {
			parts := strings.Split(k.labels, ",")
			var lb strings.Builder
			for i, kv := range parts {
				if i > 0 {
					lb.WriteByte(',')
				}
				p := strings.SplitN(kv, "=", 2)
				fmt.Fprintf(&lb, "%s=\"%s\"", p[0], p[1])
			}
			fmt.Fprintf(&sb, "%s{%s} %d\n", k.name, lb.String(), v)
		}
	}
	countersMu.RUnlock()

	// Dump gauges
	gaugesMu.RLock()
	if len(gauges) > 0 {
		gKeys := make([]gaugeKey, 0, len(gauges))
		for k := range gauges {
			gKeys = append(gKeys, k)
		}
		sort.Slice(gKeys, func(i, j int) bool {
			if gKeys[i].name != gKeys[j].name {
				return gKeys[i].name < gKeys[j].name
			}
			return gKeys[i].labels < gKeys[j].labels
		})
		for _, k := range gKeys {
			v := atomic.LoadInt64(gauges[k])
			if k.labels == "" {
				fmt.Fprintf(&sb, "%s %d\n", k.name, v)
			} else {
				parts := strings.Split(k.labels, ",")
				var lb strings.Builder
				for i, kv := range parts {
					if i > 0 {
						lb.WriteByte(',')
					}
					p := strings.SplitN(kv, "=", 2)
					fmt.Fprintf(&lb, "%s=\"%s\"", p[0], p[1])
				}
				fmt.Fprintf(&sb, "%s{%s} %d\n", k.name, lb.String(), v)
			}
		}
	}
	gaugesMu.RUnlock()

	// Dump summaries as _sum and _count pairs
	summaryMu.RLock()
	if len(summaries) > 0 {
		sKeys := make([]summaryKey, 0, len(summaries))
		for k := range summaries {
			sKeys = append(sKeys, k)
		}
		sort.Slice(sKeys, func(i, j int) bool {
			if sKeys[i].name != sKeys[j].name {
				return sKeys[i].name < sKeys[j].name
			}
			return sKeys[i].labels < sKeys[j].labels
		})
		for _, k := range sKeys {
			v := summaries[k]
			if k.labels == "" {
				fmt.Fprintf(&sb, "%s_sum %d\n%s_count %d\n", k.name, v.sum, k.name, v.count)
			} else {
				parts := strings.Split(k.labels, ",")
				var lb strings.Builder
				for i, kv := range parts {
					if i > 0 {
						lb.WriteByte(',')
					}
					p := strings.SplitN(kv, "=", 2)
					fmt.Fprintf(&lb, "%s=\"%s\"", p[0], p[1])
				}
				fmt.Fprintf(&sb, "%s_sum{%s} %d\n%s_count{%s} %d\n", k.name, lb.String(), v.sum, k.name, lb.String(), v.count)
			}
		}
	}
	summaryMu.RUnlock()
	return sb.String()
}

// --- lightweight summary (histogram-lite) ---
type summaryKey struct {
	name   string
	labels string
}
type summaryVal struct {
	sum   uint64
	count uint64
}

var (
	summaryMu sync.RWMutex
	summaries = map[summaryKey]*summaryVal{}
)

// Reset clears all in-memory metrics (counters and summaries). Intended for tests.
func Reset() {
	countersMu.Lock()
	counters = map[counterKey]*uint64{}
	countersMu.Unlock()
	summaryMu.Lock()
	summaries = map[summaryKey]*summaryVal{}
	summaryMu.Unlock()
	gaugesMu.Lock()
	gauges = map[gaugeKey]*int64{}
	gaugesMu.Unlock()
}

// --- lightweight gauge ---
type gaugeKey struct {
	name   string
	labels string
}

var (
	gaugesMu sync.RWMutex
	gauges   = map[gaugeKey]*int64{}
)

func AddGauge(name string, labels map[string]string, delta int64) {
	key := gaugeKey{name: name, labels: labelsKey(labels)}
	gaugesMu.RLock()
	p := gauges[key]
	gaugesMu.RUnlock()
	if p == nil {
		gaugesMu.Lock()
		if gauges[key] == nil {
			var v int64
			gauges[key] = &v
		}
		p = gauges[key]
		gaugesMu.Unlock()
	}
	atomic.AddInt64(p, delta)
}

func SetGauge(name string, labels map[string]string, value int64) {
	key := gaugeKey{name: name, labels: labelsKey(labels)}
	gaugesMu.Lock()
	if gauges[key] == nil {
		var v int64
		gauges[key] = &v
	}
	*gauges[key] = value
	gaugesMu.Unlock()
}
