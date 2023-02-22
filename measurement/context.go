package measurement

import "time"

type Context struct {
	StartTime time.Time
	Metrics   struct {
		ApiServerMetrics ApiServerMetrics
		EtcdMetrics      EtcdMetrics
	}
}

type ApiServerMetrics struct {
	ApiCallMetrics       ApiCallMetrics
	ResourceUsageMetrics ResourceUsageMetrics
}

type EtcdMetrics struct {
	LeaderElections           int
	ConsensusProposals        interface{}
	DbSize                    MetricStatistics[float64]
	WalSyncDuration           interface{}
	BackendCommitSyncDuration interface{}
	ResourceUsageMetrics      ResourceUsageMetrics
}

func NewContext(startTime time.Time) *Context {
	return &Context{StartTime: startTime}
}
