package measurement

import (
	"fmt"
	"time"
	measurementutil "v-bench/measurement/util"
)

type ApiCallMetrics struct {
	Metrics map[string]*ApiCallMetric
}

func (m *ApiCallMetrics) getAPICall(resource, subresource, verb, scope string) *ApiCallMetric {
	key := buildKey(resource, subresource, verb, scope)
	call, exists := m.Metrics[key]
	if !exists {
		call = &ApiCallMetric{
			Resource:    resource,
			Subresource: subresource,
			Verb:        verb,
			Scope:       scope,
		}
		m.Metrics[key] = call
	}
	return call
}

type ApiCallMetric struct {
	Resource    string                        `json:"resource"`
	Subresource string                        `json:"subresource"`
	Verb        string                        `json:"verb"`
	Scope       string                        `json:"scope"`
	Latency     measurementutil.LatencyMetric `json:"latency"`
	Count       int                           `json:"count"`
	SlowCount   int                           `json:"slowCount"`
}

func buildKey(resource, subresource, verb, scope string) string {
	return fmt.Sprintf("%s|%s|%s|%s", resource, subresource, verb, scope)
}

func (m *ApiCallMetrics) SetLatency(resource, subresource, verb, scope string, quantile float64, latency time.Duration) {
	call := m.getAPICall(resource, subresource, verb, scope)
	call.Latency.SetQuantile(quantile, latency)
}

func (m *ApiCallMetrics) GetCount(resource, subresource, verb, scope string) int {
	call := m.getAPICall(resource, subresource, verb, scope)
	return call.Count
}

func (m *ApiCallMetrics) SetCount(resource, subresource, verb, scope string, count int) {
	if count == 0 {
		return
	}
	call := m.getAPICall(resource, subresource, verb, scope)
	call.Count = count
}
