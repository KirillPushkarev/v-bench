package measurement

import "time"

type Context struct {
	StartTime time.Time
	Metrics   struct {
		ApiServerMetrics         ApiServerMetrics
		EtcdMetrics              EtcdMetrics
		ControllerManagerMetrics ControllerManagerMetrics
	}
}

type ApiServerMetrics struct {
	ApiCallMetrics       ApiCallMetrics
	ResourceUsageMetrics ResourceUsageMetrics
}

type ControllerManagerMetrics struct {
	WorkQueueDepth         MetricStatistics[float64]
	WorkQueueAdds          MetricStatistics[float64]
	WorkQueueQueueDuration MetricStatistics[float64]
	WorkQueueWorkDuration  MetricStatistics[float64]
	ApiServerMetrics       ApiCallMetrics
	ResourceUsageMetrics   ResourceUsageMetrics
}

type EtcdMetrics struct {
	LeaderElections    int
	ConsensusProposals struct {
		Committed float64
		Applied   float64
		Pending   float64
		Failed    float64
	}
	DbSize                    MetricStatistics[float64]
	WalSyncDuration           MetricStatistics[float64]
	BackendCommitSyncDuration MetricStatistics[float64]
	ResourceUsageMetrics      ResourceUsageMetrics
}

func NewContext(startTime time.Time) *Context {
	return &Context{StartTime: startTime}
}
