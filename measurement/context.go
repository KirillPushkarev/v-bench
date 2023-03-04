package measurement

import (
	"time"
	"v-bench/measurement/metric"
)

type Context struct {
	ClusterNames []string
	StartTime    time.Time
	Metrics      Metrics
}

type Metrics struct {
	ApiServerMetrics         ApiServerMetrics         `json:"api_server_metrics"`
	ControllerManagerMetrics ControllerManagerMetrics `json:"controller_manager_metrics"`
	SchedulerMetrics         SchedulerMetrics         `json:"scheduler_metrics"`
	EtcdMetrics              EtcdMetrics              `json:"etcd_metrics"`
}

type ApiServerMetrics struct {
	ApiCallMetrics       metric.ApiCallMetrics       `json:"api_call_metrics"`
	ResourceUsageMetrics metric.ResourceUsageMetrics `json:"resource_usage_metrics"`
}

type ControllerManagerMetrics struct {
	WorkQueueDepth         metric.Statistics[float64]  `json:"work_queue_depth"`
	WorkQueueAdds          metric.Statistics[float64]  `json:"work_queue_adds"`
	WorkQueueQueueDuration metric.Statistics[float64]  `json:"work_queue_queue_duration"`
	WorkQueueWorkDuration  metric.Statistics[float64]  `json:"work_queue_work_duration"`
	ApiServerMetrics       metric.ApiCallMetrics       `json:"api_server_metrics"`
	ResourceUsageMetrics   metric.ResourceUsageMetrics `json:"resource_usage_metrics"`
}

type SchedulerMetrics struct {
	SchedulingThroughput metric.Statistics[float64]  `json:"scheduling_throughput"`
	SchedulingLatency    metric.Statistics[float64]  `json:"scheduling_latency"`
	ApiServerMetrics     metric.ApiCallMetrics       `json:"api_server_metrics"`
	ResourceUsageMetrics metric.ResourceUsageMetrics `json:"resource_usage_metrics"`
}

type EtcdMetrics struct {
	LeaderElections    int `json:"leader_elections"`
	ConsensusProposals struct {
		Committed float64 `json:"committed"`
		Applied   float64 `json:"applied"`
		Pending   float64 `json:"pending"`
		Failed    float64 `json:"failed"`
	} `json:"consensus_proposals"`
	DbSize                    metric.Statistics[float64]  `json:"db_size"`
	WalSyncDuration           metric.Statistics[float64]  `json:"wal_sync_duration"`
	BackendCommitSyncDuration metric.Statistics[float64]  `json:"backend_commit_sync_duration"`
	ResourceUsageMetrics      metric.ResourceUsageMetrics `json:"resource_usage_metrics"`
}

func NewContext(clusterNames []string, startTime time.Time) *Context {
	return &Context{ClusterNames: clusterNames, StartTime: startTime}
}
