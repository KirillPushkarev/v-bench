package measurement

type ResourceUsageMetrics struct {
	CpuUsageMetric    ResourceUsageMetric
	MemoryUsageMetric ResourceUsageMetric
	ThreadUsageMetric ResourceUsageMetric
}

type ResourceUsageMetric struct {
	Perc50 float64 `json:"Perc50"`
	Perc90 float64 `json:"Perc90"`
	Perc99 float64 `json:"Perc99"`
}

// SetQuantile set quantile value.
// Only 0.5, 0.9 and 0.99 quantiles are supported.
func (metric *ResourceUsageMetric) SetQuantile(quantile float64, value float64) {
	switch quantile {
	case 0.5:
		metric.Perc50 = value
	case 0.9:
		metric.Perc90 = value
	case 0.99:
		metric.Perc99 = value
	}
}
