package measurement

type ResourceUsageMetrics struct {
	CpuUsageMetric    ResourceUsageMetric
	MemoryUsageMetric ResourceUsageMetric
	ThreadUsageMetric ResourceUsageMetric
}

type ResourceUsageMetric = MetricStatistics[float64]
