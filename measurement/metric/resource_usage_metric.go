package metric

type ResourceUsageMetrics struct {
	CpuUsageMetric    Statistics[float64]
	MemoryUsageMetric Statistics[float64]
	ThreadUsageMetric Statistics[float64]
}

type ResourceUsageMetric = Statistics[float64]
