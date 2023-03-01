package measurement

import (
	"fmt"
	"github.com/prometheus/common/model"
	"math"
	"strconv"
	"time"
	"v-bench/config"
	"v-bench/k8s"
	measurementutil "v-bench/measurement/util"
	"v-bench/prometheus/clients"
)

const (
	numK8sClients = 1

	goProcessCpuQuery            = "quantile_over_time(%.2f, avg(rate(process_cpu_seconds_total{%v}[%v]))[%v:%v])"
	goProcessRateEvaluationRange = "1m"
	goProcessRateResolution      = "1m"
	goProcessMemoryQuery         = "quantile_over_time(%.2f, avg(process_resident_memory_bytes{%v}[%v]))"
	goProcessThreadQuery         = "quantile_over_time(%.2f, avg(go_goroutines{%v}[%v]))"

	// apiServerLatencyQuery measures 99th percentile of API call latency over given period of time
	// apiServerLatencyQuery: placeholders should be replaced with (1) quantile (2) apiServerApiCallFilters and (3) query window size.
	apiServerLatencyQuery         = "histogram_quantile(%.2f, sum(rate(apiserver_request_duration_seconds_bucket{%v}[%v])) by (verb, resource, subresource, scope, le))"
	apiServerThroughputQuery      = "quantile_over_time(%.2f, avg(rate(apiserver_request_total{%v}[%v])) by (verb, resource, subresource, scope, code, le)[%v:%v])"
	apiServerApiCallFilters       = `job="apiserver", verb!="WATCH", subresource!~"log|exec|portforward|attach|proxy"`
	apiServerCpuQuery             = goProcessCpuQuery
	apiServerRateEvaluationRange  = goProcessRateEvaluationRange
	apiServerRateResolution       = goProcessRateResolution
	apiServerMemoryQuery          = goProcessMemoryQuery
	apiServerThreadQuery          = goProcessThreadQuery
	apiServerResourceUsageFilters = `job="apiserver"`

	controllerManagerWorkQueueAddsQuery          = "quantile_over_time(%.2f, avg(rate(workqueue_adds_total{%v}[%v]))[%v:%v])"
	controllerManagerWorkQueueDepthQuery         = "quantile_over_time(%.2f, avg(rate(workqueue_depth{%v}[%v]))[%v:%v])"
	controllerManagerWorkQueueQueueDurationQuery = "histogram_quantile(%.2f, sum(rate(workqueue_queue_duration_seconds_bucket{%v}[%v])) by (le))"
	controllerManagerWorkQueueWorkDurationQuery  = "histogram_quantile(%.2f, sum(rate(workqueue_work_duration_seconds_bucket{%v}[%v])) by (le))"
	controllerManagerToApiServerLatencyQuery     = "histogram_quantile(%.2f, sum(rate(rest_client_request_duration_seconds_bucket{%v}[%v])) by (verb, le))"
	controllerManagerToApiServerThroughputQuery  = "quantile_over_time(%.2f, avg(rate(rest_client_requests_total{%v}[%v])) by (method, code)[%v:%v])"
	controllerManagerCpuQuery                    = goProcessCpuQuery
	controllerManagerRateEvaluationRange         = goProcessRateEvaluationRange
	controllerManagerRateResolution              = goProcessRateResolution
	controllerManagerMemoryQuery                 = goProcessMemoryQuery
	controllerManagerThreadQuery                 = goProcessThreadQuery
	controllerManagerCommonFilters               = `job="kube-controller-manager"`

	schedulerSchedulingLatencyQuery     = "histogram_quantile(%.2f, sum(rate(scheduler_scheduling_algorithm_duration_seconds_bucket{%v}[%v])))"
	schedulerSchedulingThroughputQuery  = "quantile_over_time(%.2f, avg(rate(scheduler_scheduling_algorithm_duration_seconds_count{%v}[%v]))[%v:%v])"
	schedulerToApiServerLatencyQuery    = "histogram_quantile(%.2f, sum(rate(rest_client_request_duration_seconds_bucket{%v}[%v])) by (verb, le))"
	schedulerToApiServerThroughputQuery = "quantile_over_time(%.2f, avg(rate(rest_client_requests_total{%v}[%v])) by (method, code)[%v:%v])"
	schedulerCpuQuery                   = goProcessCpuQuery
	schedulerRateEvaluationRange        = goProcessRateEvaluationRange
	schedulerRateResolution             = goProcessRateResolution
	schedulerMemoryQuery                = goProcessMemoryQuery
	schedulerThreadQuery                = goProcessThreadQuery
	schedulerCommonFilters              = `job="kube-scheduler"`

	etcdLeaderElectionsQuery    = "max(increase(etcd_server_leader_changes_seen_total{%v}[%v]))"
	etcdDbSizeQuery             = "max(quantile_over_time(%.2f, etcd_mvcc_db_total_size_in_bytes{%v}[%v]))"
	etcdWalSyncQuery            = "histogram_quantile(%.2f, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket{%v}[%v])) by (le))"
	etcdBackendCommitSyncQuery  = "histogram_quantile(%.2f, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket{%v}[%v])) by (le))"
	etcdProposalsCommittedQuery = "sum(rate(etcd_server_proposals_committed_total{%v}[%v])"
	etcdProposalsAppliedQuery   = "sum(rate(etcd_server_proposals_applied_total{%v}[%v])"
	etcdProposalsPendingQuery   = "sum(quantile_over_time(0.5, etcd_server_proposals_pending{%v}[%v]))"
	etcdProposalsFailedQuery    = "sum(rate(etcd_server_proposals_failed_total{%v}[%v])"
	etcdCpuQuery                = goProcessCpuQuery
	etcdRateEvaluationRange     = goProcessRateEvaluationRange
	etcdRateResolution          = goProcessRateResolution
	etcdMemoryQuery             = goProcessMemoryQuery
	etcdThreadQuery             = goProcessThreadQuery
	etcdCommonFilters           = `job="etcd"`
)

var quantiles = []float64{0.5, 0.9, 0.99}

type MetricCollector struct {
}

func NewMetricCollector() *MetricCollector {
	return &MetricCollector{}
}

func (*MetricCollector) CollectMetrics(benchmarkConfig config.TestConfig, context *Context) {
	var pc clients.Client
	prometheusFramework, err := k8s.NewFramework(benchmarkConfig.RootKubeConfigPath, numK8sClients)
	if err != nil {
		fmt.Printf("k8s framework creation error: %v", err)
	}
	pc = clients.NewInClusterPrometheusClient(prometheusFramework.GetClientSets().GetClient())
	executor := NewPrometheusQueryExecutor(pc)
	endTime := time.Now()
	duration := endTime.Sub(context.StartTime)
	durationInPromFormat := measurementutil.ToPrometheusTime(duration)

	collectApiServerMetrics(context, executor, endTime, durationInPromFormat)
	collectControllerManagerMetrics(context, executor, endTime, durationInPromFormat)
	collectSchedulerMetrics(context, executor, endTime, durationInPromFormat)
	collectEtcdMetrics(context, executor, endTime, durationInPromFormat)
	//collectOverallControlPlaneMetrics(durationInPromFormat, executor, endTime, err, context)
}

func collectApiServerMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	apiServerMetrics := &context.Metrics.ApiServerMetrics

	var throughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerThroughputQuery, q, apiServerApiCallFilters, apiServerRateEvaluationRange, durationInPromFormat, apiServerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		throughputSamples = append(throughputSamples, samples...)
	}

	var latencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerLatencyQuery, q, apiServerApiCallFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		latencySamples = append(latencySamples, samples...)
	}

	apiCallMetrics, err := apiCallMetricsFromSamples(throughputSamples, latencySamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	apiServerMetrics.ApiCallMetrics = *apiCallMetrics

	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerCpuQuery, q, apiServerResourceUsageFilters, apiServerRateEvaluationRange, durationInPromFormat, apiServerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		cpuSamples = append(cpuSamples, samples...)
	}

	var memorySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerMemoryQuery, q, apiServerResourceUsageFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		memorySamples = append(memorySamples, samples...)
	}

	var threadSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerThreadQuery, q, apiServerResourceUsageFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		threadSamples = append(threadSamples, samples...)
	}

	resourceUsageMetrics, err := resourceUsageMetricsFromSamples(cpuSamples, memorySamples, threadSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	apiServerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func collectControllerManagerMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	controllerManagerMetrics := &context.Metrics.ControllerManagerMetrics

	var queueDepthSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueDepthQuery, q, controllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueDepthSamples = append(queueDepthSamples, samples...)
	}
	queueSizeStatistics, err := metricStatisticsFromSamples[float64](queueDepthSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	controllerManagerMetrics.WorkQueueDepth = *queueSizeStatistics

	var queueAddSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueAddsQuery, q, controllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueAddSamples = append(queueAddSamples, samples...)
	}
	queueAddStatistics, err := metricStatisticsFromSamples[float64](queueAddSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	controllerManagerMetrics.WorkQueueAdds = *queueAddStatistics

	var queueQueueDurationSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueQueueDurationQuery, q, controllerManagerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueQueueDurationSamples = append(queueQueueDurationSamples, samples...)
	}
	queueQueueDurationStatistics, err := metricStatisticsFromSamples[float64](queueQueueDurationSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	controllerManagerMetrics.WorkQueueQueueDuration = *queueQueueDurationStatistics

	var queueWorkDurationSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueWorkDurationQuery, q, controllerManagerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueQueueDurationSamples = append(queueQueueDurationSamples, samples...)
	}
	queueWorkDurationStatistics, err := metricStatisticsFromSamples[float64](queueWorkDurationSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	controllerManagerMetrics.WorkQueueWorkDuration = *queueWorkDurationStatistics

	var throughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerToApiServerThroughputQuery, q, controllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		throughputSamples = append(throughputSamples, samples...)
	}

	var latencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerToApiServerLatencyQuery, q, controllerManagerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		latencySamples = append(latencySamples, samples...)
	}

	apiCallMetrics, err := apiCallMetricsFromSamples(throughputSamples, latencySamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	controllerManagerMetrics.ApiServerMetrics = *apiCallMetrics

	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerCpuQuery, q, controllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		cpuSamples = append(cpuSamples, samples...)
	}

	var memorySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerMemoryQuery, q, controllerManagerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		memorySamples = append(memorySamples, samples...)
	}

	var threadSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerThreadQuery, q, controllerManagerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		threadSamples = append(threadSamples, samples...)
	}

	resourceUsageMetrics, err := resourceUsageMetricsFromSamples(cpuSamples, memorySamples, threadSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	controllerManagerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func collectSchedulerMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	schedulerMetrics := &context.Metrics.SchedulerMetrics

	var schedulingThroughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerSchedulingThroughputQuery, q, schedulerCommonFilters, schedulerRateEvaluationRange, durationInPromFormat, schedulerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		schedulingThroughputSamples = append(schedulingThroughputSamples, samples...)
	}
	schedulingThroughputStatistics, err := metricStatisticsFromSamples[float64](schedulingThroughputSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	schedulerMetrics.SchedulingThroughput = *schedulingThroughputStatistics

	var schedulingLatencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerSchedulingLatencyQuery, q, schedulerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		schedulingLatencySamples = append(schedulingLatencySamples, samples...)
	}
	schedulingLatencyStatistics, err := metricStatisticsFromSamples[float64](schedulingLatencySamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	schedulerMetrics.SchedulingLatency = *schedulingLatencyStatistics

	var throughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerToApiServerThroughputQuery, q, schedulerCommonFilters, schedulerRateEvaluationRange, durationInPromFormat, schedulerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		throughputSamples = append(throughputSamples, samples...)
	}

	var latencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerToApiServerLatencyQuery, q, schedulerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		latencySamples = append(latencySamples, samples...)
	}

	apiCallMetrics, err := apiCallMetricsFromSamples(throughputSamples, latencySamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	schedulerMetrics.ApiServerMetrics = *apiCallMetrics

	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerCpuQuery, q, schedulerCommonFilters, schedulerRateEvaluationRange, durationInPromFormat, schedulerRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		cpuSamples = append(cpuSamples, samples...)
	}

	var memorySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerMemoryQuery, q, schedulerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		memorySamples = append(memorySamples, samples...)
	}

	var threadSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerThreadQuery, q, schedulerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		threadSamples = append(threadSamples, samples...)
	}

	resourceUsageMetrics, err := resourceUsageMetricsFromSamples(cpuSamples, memorySamples, threadSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	schedulerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func collectEtcdMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	etcdMetrics := &context.Metrics.EtcdMetrics

	leaderElectionsQuery := fmt.Sprintf(etcdLeaderElectionsQuery, etcdCommonFilters, durationInPromFormat)
	leaderElectionsSamples, err := executor.Query(leaderElectionsQuery, endTime)
	if err != nil {
		fmt.Printf("prometheus query execution error: %v", err)
	}
	etcdMetrics.LeaderElections = int(math.Round(float64(leaderElectionsSamples[0].Value)))

	var dbSizeSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdDbSizeQuery, q, etcdCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		dbSizeSamples = append(dbSizeSamples, samples...)
	}
	dbSizeStatistics, err := metricStatisticsFromSamples[float64](dbSizeSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	etcdMetrics.DbSize = *dbSizeStatistics

	var walSyncSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdWalSyncQuery, q, etcdCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		walSyncSamples = append(walSyncSamples, samples...)
	}
	walSyncStatistics, err := metricStatisticsFromSamples[float64](walSyncSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	etcdMetrics.WalSyncDuration = *walSyncStatistics

	var backendCommitSyncSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdBackendCommitSyncQuery, q, etcdCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		backendCommitSyncSamples = append(backendCommitSyncSamples, samples...)
	}
	backendCommitSyncStatistics, err := metricStatisticsFromSamples[float64](backendCommitSyncSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	etcdMetrics.BackendCommitSyncDuration = *backendCommitSyncStatistics

	proposalsMetrics := etcdMetrics.ConsensusProposals
	proposalsCommittedQuery := fmt.Sprintf(etcdProposalsCommittedQuery, etcdCommonFilters, durationInPromFormat)
	proposalsCommittedSamples, err := executor.Query(proposalsCommittedQuery, endTime)
	if err != nil {
		fmt.Printf("prometheus query execution error: %v", err)
	}
	proposalsMetrics.Committed = float64(proposalsCommittedSamples[0].Value)

	proposalsAppliedQuery := fmt.Sprintf(etcdProposalsAppliedQuery, etcdCommonFilters, durationInPromFormat)
	proposalsAppliedSamples, err := executor.Query(proposalsAppliedQuery, endTime)
	if err != nil {
		fmt.Printf("prometheus query execution error: %v", err)
	}
	proposalsMetrics.Applied = float64(proposalsAppliedSamples[0].Value)
	proposalsPendingQuery := fmt.Sprintf(etcdProposalsPendingQuery, etcdCommonFilters, durationInPromFormat)
	proposalsPendingSamples, err := executor.Query(proposalsPendingQuery, endTime)
	if err != nil {
		fmt.Printf("prometheus query execution error: %v", err)
	}
	proposalsMetrics.Pending = float64(proposalsPendingSamples[0].Value)
	proposalsFailedQuery := fmt.Sprintf(etcdProposalsFailedQuery, etcdCommonFilters, durationInPromFormat)
	proposalsFailedSamples, err := executor.Query(proposalsFailedQuery, endTime)
	if err != nil {
		fmt.Printf("prometheus query execution error: %v", err)
	}
	proposalsMetrics.Failed = float64(proposalsFailedSamples[0].Value)

	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdCpuQuery, q, etcdCommonFilters, etcdRateEvaluationRange, durationInPromFormat, etcdRateResolution)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		cpuSamples = append(cpuSamples, samples...)
	}

	var memorySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdMemoryQuery, q, etcdCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		memorySamples = append(memorySamples, samples...)
	}

	var threadSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdThreadQuery, q, etcdCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		threadSamples = append(threadSamples, samples...)
	}

	resourceUsageMetrics, err := resourceUsageMetricsFromSamples(cpuSamples, memorySamples, threadSamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	etcdMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func apiCallMetricsFromSamples(throughputSamples []*model.Sample, latencySamples []*model.Sample) (*ApiCallMetrics, error) {
	extractLabelValues := func(sample *model.Sample) []string {
		return []string{
			string(sample.Metric["resource"]),
			string(sample.Metric["subresource"]),
			string(sample.Metric["verb"]),
			string(sample.Metric["scope"]),
		}
	}

	m := &ApiCallMetrics{MetricByKey: make(map[string]*ApiCallMetric)}

	for _, sample := range throughputSamples {
		labels := extractLabelValues(sample)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		throughput := float64(sample.Value)
		m.SetThroughput(labels, quantile, throughput)
	}

	for _, sample := range latencySamples {
		labels := extractLabelValues(sample)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		latency := time.Duration(float64(sample.Value) * float64(time.Second))
		m.SetLatency(labels, quantile, latency)
	}

	return m, nil
}

func resourceUsageMetricsFromSamples(cpuSamples []*model.Sample, memorySamples []*model.Sample, threadSamples []*model.Sample) (*ResourceUsageMetrics, error) {
	m := &ResourceUsageMetrics{CpuUsageMetric: ResourceUsageMetric{}, MemoryUsageMetric: ResourceUsageMetric{}, ThreadUsageMetric: ResourceUsageMetric{}}

	for _, sample := range cpuSamples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := float64(sample.Value)
		m.CpuUsageMetric.SetQuantile(quantile, value)
	}

	for _, sample := range memorySamples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := float64(sample.Value)
		m.MemoryUsageMetric.SetQuantile(quantile, value)
	}

	for _, sample := range threadSamples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := float64(sample.Value)
		m.ThreadUsageMetric.SetQuantile(quantile, value)
	}

	return m, nil
}

func metricStatisticsFromSamples[T int | float64](samples []*model.Sample) (*MetricStatistics[T], error) {
	m := &MetricStatistics[T]{}

	for _, sample := range samples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := T(sample.Value)
		m.SetQuantile(quantile, value)
	}

	return m, nil
}
