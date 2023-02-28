package measurement

import (
	"strings"
	"time"
	measurementutil "v-bench/measurement/util"
)

type ApiCallMetrics struct {
	MetricByKey map[string]*ApiCallMetric
}

type ApiCallMetric struct {
	Labels     []string                      `json:"labels"`
	Throughput MetricStatistics[float64]     `json:"throughput"`
	Latency    measurementutil.LatencyMetric `json:"latency"`
}

func (m *ApiCallMetrics) SetThroughput(labels []string, quantile float64, throughput float64) {
	call := m.getAPICall(labels)
	call.Throughput.SetQuantile(quantile, throughput)
}

func (m *ApiCallMetrics) SetLatency(labels []string, quantile float64, latency time.Duration) {
	call := m.getAPICall(labels)
	call.Latency.SetQuantile(quantile, latency)
}

func (m *ApiCallMetrics) getAPICall(labels []string) *ApiCallMetric {
	key := buildKey(labels)
	call, exists := m.MetricByKey[key]
	if !exists {
		call = &ApiCallMetric{
			Labels: labels,
		}
		m.MetricByKey[key] = call
	}
	return call
}

func buildKey(labels []string) string {
	return strings.Join(labels, "|")
}
