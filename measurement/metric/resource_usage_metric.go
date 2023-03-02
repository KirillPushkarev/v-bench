package metric

type ResourceUsageMetrics struct {
	CpuUsage    Statistics[float64] `json:"cpu_usage"`
	MemoryUsage Statistics[float64] `json:"memory_usage"`
	ThreadUsage Statistics[float64] `json:"thread_usage"`
}
