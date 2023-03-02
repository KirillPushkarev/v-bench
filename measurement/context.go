package measurement

import (
	"time"
	"v-bench/measurement/metric"
)

type Context struct {
	StartTime time.Time
	Metrics   struct {
		ApiServerMetrics         ApiServerMetrics
		EtcdMetrics              EtcdMetrics
		ControllerManagerMetrics ControllerManagerMetrics
		SchedulerMetrics         SchedulerMetrics
	}
}

type ApiServerMetrics struct {
	ApiCallMetrics       metric.ApiCallMetrics
	ResourceUsageMetrics metric.ResourceUsageMetrics
}

type ControllerManagerMetrics struct {
	WorkQueueDepth         metric.Statistics[float64]
	WorkQueueAdds          metric.Statistics[float64]
	WorkQueueQueueDuration metric.Statistics[float64]
	WorkQueueWorkDuration  metric.Statistics[float64]
	ApiServerMetrics       metric.ApiCallMetrics
	ResourceUsageMetrics   metric.ResourceUsageMetrics
}

type SchedulerMetrics struct {
	SchedulingThroughput metric.Statistics[float64]
	SchedulingLatency    metric.Statistics[float64]
	ApiServerMetrics     metric.ApiCallMetrics
	ResourceUsageMetrics metric.ResourceUsageMetrics
}

type EtcdMetrics struct {
	LeaderElections    int
	ConsensusProposals struct {
		Committed float64
		Applied   float64
		Pending   float64
		Failed    float64
	}
	DbSize                    metric.Statistics[float64]
	WalSyncDuration           metric.Statistics[float64]
	BackendCommitSyncDuration metric.Statistics[float64]
	ResourceUsageMetrics      metric.ResourceUsageMetrics
}

func NewContext(startTime time.Time) *Context {
	return &Context{StartTime: startTime}
}
