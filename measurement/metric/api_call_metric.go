package metric

type ApiCallMetrics struct {
	MetricByKey map[string]*ApiCallMetric `json:"metric_by_key"`
}

type ApiCallMetric struct {
	Labels     map[string]string   `json:"labels"`
	Throughput Statistics[float64] `json:"throughput"`
	Latency    LatencyMetric       `json:"latency"`
}

func (m *ApiCallMetrics) SetThroughput(key string, labels map[string]string, quantile float64, throughput float64) {
	call := m.getAPICall(key, labels)
	call.Throughput.SetQuantile(quantile, throughput)
}

func (m *ApiCallMetrics) SetLatency(key string, labels map[string]string, quantile float64, latency float64) {
	call := m.getAPICall(key, labels)
	call.Latency.SetQuantile(quantile, latency)
}

func (m *ApiCallMetrics) getAPICall(key string, labels map[string]string) *ApiCallMetric {
	call, exists := m.MetricByKey[key]
	if !exists {
		call = &ApiCallMetric{
			Labels: labels,
		}
		m.MetricByKey[key] = call
	}
	return call
}
