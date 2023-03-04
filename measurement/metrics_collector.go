package measurement

import (
	"fmt"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
	"strings"
	"time"
	"v-bench/k8s"
	"v-bench/measurement/metric"
	measurementutil "v-bench/measurement/util"
	"v-bench/prometheus/clients"
)

const (
	numK8sClients = 1

	clusterLabelName = "cluster"

	goProcessCpuQuery            = "quantile_over_time(%.2f, avg(rate(process_cpu_seconds_total{%v}[%v]))[%v:%v])"
	goProcessRateEvaluationRange = "1m"
	goProcessRateResolution      = "1m"
	goProcessMemoryQuery         = "avg(quantile_over_time(%.2f, process_resident_memory_bytes{%v}[%v]))"
	goProcessThreadQuery         = "avg(quantile_over_time(%.2f, go_goroutines{%v}[%v]))"

	// apiServerLatencyQuery measures q-quantile of API call latency over given period of time
	// apiServerLatencyQuery: placeholders should be replaced with (1) quantile (2) apiServerApiCallFilters and (3) query window size.
	apiServerLatencyQuery         = "histogram_quantile(%.2f, sum(rate(apiserver_request_duration_seconds_bucket{%v}[%v])) by (verb, resource, subresource, scope, le))"
	apiServerThroughputQuery      = "quantile_over_time(%.2f, avg(rate(apiserver_request_total{%v}[%v])) by (verb, resource, subresource, scope, le)[%v:%v])"
	apiServerApiCallFilters       = `%v=~"%v", job="apiserver", verb=~"GET|LIST|POST|PUT|PATCH|DELETE", resource=~"pods|deployments|replicationcontrollers|statefulsets|daemonsets|jobs|cronjobs|services|configmaps|secrets|volumes|persistentvolumeclaims|persistentvolumes|nodes|namespaces", subresource!~"log|exec|portforward|attach|proxy"`
	apiServerRateEvaluationRange  = goProcessRateEvaluationRange
	apiServerRateResolution       = goProcessRateResolution
	apiServerResourceUsageFilters = `%v=~"%v", job="apiserver"`

	controllerManagerWorkQueueAddsQuery          = "quantile_over_time(%.2f, avg(rate(workqueue_adds_total{%v}[%v]))[%v:%v])"
	controllerManagerWorkQueueDepthQuery         = "quantile_over_time(%.2f, avg(rate(workqueue_depth{%v}[%v]))[%v:%v])"
	controllerManagerWorkQueueQueueDurationQuery = "histogram_quantile(%.2f, sum(rate(workqueue_queue_duration_seconds_bucket{%v}[%v])) by (le))"
	controllerManagerWorkQueueWorkDurationQuery  = "histogram_quantile(%.2f, sum(rate(workqueue_work_duration_seconds_bucket{%v}[%v])) by (le))"
	controllerManagerToApiServerLatencyQuery     = "histogram_quantile(%.2f, sum(rate(rest_client_request_duration_seconds_bucket{%v}[%v])) by (verb, le))"
	controllerManagerToApiServerThroughputQuery  = "quantile_over_time(%.2f, avg(rate(rest_client_requests_total{%v}[%v])) by (method)[%v:%v])"
	controllerManagerRateEvaluationRange         = goProcessRateEvaluationRange
	controllerManagerRateResolution              = goProcessRateResolution
	controllerManagerCommonFilters               = `%v=~"%v", job="kube-controller-manager"`

	schedulerSchedulingLatencyQuery     = "histogram_quantile(%.2f, sum(rate(scheduler_scheduling_algorithm_duration_seconds_bucket{%v}[%v])))"
	schedulerSchedulingThroughputQuery  = "quantile_over_time(%.2f, avg(rate(scheduler_scheduling_algorithm_duration_seconds_count{%v}[%v]))[%v:%v])"
	schedulerToApiServerLatencyQuery    = "histogram_quantile(%.2f, sum(rate(rest_client_request_duration_seconds_bucket{%v}[%v])) by (verb, le))"
	schedulerToApiServerThroughputQuery = "quantile_over_time(%.2f, avg(rate(rest_client_requests_total{%v}[%v])) by (method)[%v:%v])"
	schedulerRateEvaluationRange        = goProcessRateEvaluationRange
	schedulerRateResolution             = goProcessRateResolution
	schedulerCommonFilters              = `%v=~"%v", job="kube-scheduler"`

	etcdLeaderElectionsQuery    = "max(increase(etcd_server_leader_changes_seen_total{%v}[%v]))"
	etcdDbSizeQuery             = "max(quantile_over_time(%.2f, etcd_mvcc_db_total_size_in_bytes{%v}[%v]))"
	etcdWalSyncQuery            = "histogram_quantile(%.2f, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket{%v}[%v])) by (le))"
	etcdBackendCommitSyncQuery  = "histogram_quantile(%.2f, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket{%v}[%v])) by (le))"
	etcdProposalsCommittedQuery = "sum(rate(etcd_server_proposals_committed_total{%v}[%v]))"
	etcdProposalsAppliedQuery   = "sum(rate(etcd_server_proposals_applied_total{%v}[%v]))"
	etcdProposalsPendingQuery   = "sum(avg_over_time(etcd_server_proposals_pending{%v}[%v]))"
	etcdProposalsFailedQuery    = "sum(rate(etcd_server_proposals_failed_total{%v}[%v]))"
	etcdCommonFilters           = `%v=~"%v", job="etcd"`
)

type UnitConverter[T int | float64 | time.Duration] func(sample model.SampleValue) T

var (
	quantiles = []float64{0.5, 0.90, 0.99}

	rateConverter UnitConverter[float64] = func(sample model.SampleValue) float64 {
		return math.Round(float64(sample)*1000) / 1000
	}
	durationConverter UnitConverter[float64] = func(sample model.SampleValue) float64 {
		return math.Round(float64(sample) * 1000)
	}
	memoryConverter UnitConverter[float64] = func(sample model.SampleValue) float64 {
		return math.Round(float64(sample)/1024/1024*1000) / 1000
	}
)

type CollectConfig struct {
	ShouldCollectScheduler bool
}

func NewCollectConfig(shouldCollectScheduler bool) *CollectConfig {
	return &CollectConfig{ShouldCollectScheduler: shouldCollectScheduler}
}

type MetricCollector struct {
	executor *PrometheusQueryExecutor
}

func NewMetricCollector(kubeConfigPath string) (*MetricCollector, error) {
	prometheusFramework, err := k8s.NewFramework(kubeConfigPath, numK8sClients)
	if err != nil {
		return nil, fmt.Errorf("k8s framework creation error: %v", err)
	}
	pc := clients.NewInClusterPrometheusClient(prometheusFramework.GetClientSets().GetClient())
	executor := NewPrometheusQueryExecutor(pc)

	return &MetricCollector{executor: executor}, nil
}

type MetricFilters struct {
	ApiServerApiCallFilters        string
	ApiServerResourceUsageFilters  string
	ControllerManagerCommonFilters string
	SchedulerCommonFilters         string
	EtcdCommonFilters              string
}

func (mc *MetricCollector) CollectMetrics(context *Context, collectConfig *CollectConfig) {
	metricFilters := MetricFilters{
		ApiServerApiCallFilters:        fmt.Sprintf(apiServerApiCallFilters, clusterLabelName, strings.Join(context.ClusterNames, "|")),
		ApiServerResourceUsageFilters:  fmt.Sprintf(apiServerResourceUsageFilters, clusterLabelName, strings.Join(context.ClusterNames, "|")),
		ControllerManagerCommonFilters: fmt.Sprintf(controllerManagerCommonFilters, clusterLabelName, strings.Join(context.ClusterNames, "|")),
		SchedulerCommonFilters:         fmt.Sprintf(schedulerCommonFilters, clusterLabelName, strings.Join(context.ClusterNames, "|")),
		EtcdCommonFilters:              fmt.Sprintf(etcdCommonFilters, clusterLabelName, strings.Join(context.ClusterNames, "|")),
	}

	endTime := time.Now()
	duration := endTime.Sub(context.StartTime)
	durationInPromFormat := measurementutil.ToPrometheusTime(duration)

	mc.collectApiServerMetrics(context, collectConfig, metricFilters, endTime, durationInPromFormat)
	mc.collectControllerManagerMetrics(context, collectConfig, metricFilters, endTime, durationInPromFormat)
	if collectConfig.ShouldCollectScheduler {
		mc.collectSchedulerMetrics(context, collectConfig, metricFilters, endTime, durationInPromFormat)
	}
	mc.collectEtcdMetrics(context, collectConfig, metricFilters, endTime, durationInPromFormat)
	//collectOverallControlPlaneMetrics(durationInPromFormat, executor, endTime, err, context)
}

func (mc *MetricCollector) collectApiServerMetrics(context *Context, config *CollectConfig, filters MetricFilters, endTime time.Time, durationInPromFormat string) {
	apiServerMetrics := &context.Metrics.ApiServerMetrics

	var throughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerThroughputQuery, q, filters.ApiServerApiCallFilters, apiServerRateEvaluationRange, durationInPromFormat, apiServerRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		throughputSamples = append(throughputSamples, samples...)
	}

	var latencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerLatencyQuery, q, filters.ApiServerApiCallFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		latencySamples = append(latencySamples, samples...)
	}

	apiCallMetrics, err := apiServerCallInternalMetricsFromSamples(throughputSamples, latencySamples)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		apiServerMetrics.ApiCallMetrics = *apiCallMetrics
	}

	resourceUsageMetrics := mc.queryForResourceUsage(filters.ApiServerResourceUsageFilters, durationInPromFormat, endTime)
	apiServerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func logQueryExecutionError(err error, query string) {
	log.Errorf("prometheus query execution error: %v, original query: %v", err, query)
}

func (mc *MetricCollector) collectControllerManagerMetrics(context *Context, config *CollectConfig, filters MetricFilters, endTime time.Time, durationInPromFormat string) {
	controllerManagerMetrics := &context.Metrics.ControllerManagerMetrics

	var queueDepthSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueDepthQuery, q, filters.ControllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueDepthSamples = append(queueDepthSamples, samples...)
	}
	queueDepthConverter := func(sample model.SampleValue) float64 {
		return math.Round(float64(sample))
	}
	queueDepthStatistics, err := metricStatisticsFromSamples[float64](queueDepthSamples, queueDepthConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		controllerManagerMetrics.WorkQueueDepth = *queueDepthStatistics
	}

	var queueAddSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueAddsQuery, q, filters.ControllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueAddSamples = append(queueAddSamples, samples...)
	}
	queueAddStatistics, err := metricStatisticsFromSamples[float64](queueAddSamples, rateConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		controllerManagerMetrics.WorkQueueAdds = *queueAddStatistics
	}

	var queueQueueDurationSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueQueueDurationQuery, q, filters.ControllerManagerCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueQueueDurationSamples = append(queueQueueDurationSamples, samples...)
	}
	queueQueueDurationStatistics, err := metricStatisticsFromSamples[float64](queueQueueDurationSamples, durationConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		controllerManagerMetrics.WorkQueueQueueDuration = *queueQueueDurationStatistics
	}

	var queueWorkDurationSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerWorkQueueWorkDurationQuery, q, filters.ControllerManagerCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		queueQueueDurationSamples = append(queueQueueDurationSamples, samples...)
	}
	queueWorkDurationStatistics, err := metricStatisticsFromSamples[float64](queueWorkDurationSamples, durationConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		controllerManagerMetrics.WorkQueueWorkDuration = *queueWorkDurationStatistics
	}

	var apiServerThroughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerToApiServerThroughputQuery, q, filters.ControllerManagerCommonFilters, controllerManagerRateEvaluationRange, durationInPromFormat, controllerManagerRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		apiServerThroughputSamples = append(apiServerThroughputSamples, samples...)
	}

	var apiServerLatencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(controllerManagerToApiServerLatencyQuery, q, filters.ControllerManagerCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		apiServerLatencySamples = append(apiServerLatencySamples, samples...)
	}

	apiCallMetrics, err := apiServerCallExternalMetricsFromSamples(apiServerThroughputSamples, apiServerLatencySamples)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		controllerManagerMetrics.ApiServerMetrics = *apiCallMetrics
	}

	resourceUsageMetrics := mc.queryForResourceUsage(filters.ControllerManagerCommonFilters, durationInPromFormat, endTime)
	controllerManagerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func (mc *MetricCollector) collectSchedulerMetrics(context *Context, config *CollectConfig, filters MetricFilters, endTime time.Time, durationInPromFormat string) {
	schedulerMetrics := &context.Metrics.SchedulerMetrics

	var schedulingThroughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerSchedulingThroughputQuery, q, filters.SchedulerCommonFilters, schedulerRateEvaluationRange, durationInPromFormat, schedulerRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		schedulingThroughputSamples = append(schedulingThroughputSamples, samples...)
	}
	schedulingThroughputStatistics, err := metricStatisticsFromSamples[float64](schedulingThroughputSamples, rateConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		schedulerMetrics.SchedulingThroughput = *schedulingThroughputStatistics
	}

	var schedulingLatencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerSchedulingLatencyQuery, q, filters.SchedulerCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		schedulingLatencySamples = append(schedulingLatencySamples, samples...)
	}
	schedulingLatencyStatistics, err := metricStatisticsFromSamples[float64](schedulingLatencySamples, durationConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		schedulerMetrics.SchedulingLatency = *schedulingLatencyStatistics
	}

	var apiServerThroughputSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerToApiServerThroughputQuery, q, filters.SchedulerCommonFilters, schedulerRateEvaluationRange, durationInPromFormat, schedulerRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		apiServerThroughputSamples = append(apiServerThroughputSamples, samples...)
	}

	var apiServerLatencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(schedulerToApiServerLatencyQuery, q, filters.SchedulerCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		apiServerLatencySamples = append(apiServerLatencySamples, samples...)
	}

	apiCallMetrics, err := apiServerCallExternalMetricsFromSamples(apiServerThroughputSamples, apiServerLatencySamples)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		schedulerMetrics.ApiServerMetrics = *apiCallMetrics
	}

	resourceUsageMetrics := mc.queryForResourceUsage(filters.SchedulerCommonFilters, durationInPromFormat, endTime)
	schedulerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func (mc *MetricCollector) collectEtcdMetrics(context *Context, config *CollectConfig, filters MetricFilters, endTime time.Time, durationInPromFormat string) {
	etcdMetrics := &context.Metrics.EtcdMetrics

	leaderElectionsQuery := fmt.Sprintf(etcdLeaderElectionsQuery, filters.EtcdCommonFilters, durationInPromFormat)
	leaderElectionsSamples, err := mc.executor.Query(leaderElectionsQuery, endTime)
	if err != nil {
		logQueryExecutionError(err, leaderElectionsQuery)
	} else {
		etcdMetrics.LeaderElections = int(math.Round(float64(leaderElectionsSamples[0].Value)))
	}

	var dbSizeSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdDbSizeQuery, q, filters.EtcdCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		dbSizeSamples = append(dbSizeSamples, samples...)
	}
	dbSizeStatistics, err := metricStatisticsFromSamples[float64](dbSizeSamples, memoryConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		etcdMetrics.DbSize = *dbSizeStatistics
	}

	var walSyncSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdWalSyncQuery, q, filters.EtcdCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		walSyncSamples = append(walSyncSamples, samples...)
	}
	walSyncStatistics, err := metricStatisticsFromSamples[float64](walSyncSamples, durationConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		etcdMetrics.WalSyncDuration = *walSyncStatistics
	}

	var backendCommitSyncSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdBackendCommitSyncQuery, q, filters.EtcdCommonFilters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
			continue
		}

		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		backendCommitSyncSamples = append(backendCommitSyncSamples, samples...)
	}
	backendCommitSyncStatistics, err := metricStatisticsFromSamples[float64](backendCommitSyncSamples, durationConverter)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	} else {
		etcdMetrics.BackendCommitSyncDuration = *backendCommitSyncStatistics
	}

	proposalsMetrics := etcdMetrics.ConsensusProposals

	proposalsCommittedSamples, err := mc.queryForProposals(etcdProposalsCommittedQuery, filters, durationInPromFormat, endTime)
	if err == nil {
		proposalsMetrics.Committed = float64(proposalsCommittedSamples[0].Value)
	}
	proposalsAppliedSamples, err := mc.queryForProposals(etcdProposalsAppliedQuery, filters, durationInPromFormat, endTime)
	if err == nil {
		proposalsMetrics.Applied = float64(proposalsAppliedSamples[0].Value)
	}
	proposalsPendingSamples, err := mc.queryForProposals(etcdProposalsPendingQuery, filters, durationInPromFormat, endTime)
	if err == nil {
		proposalsMetrics.Pending = float64(proposalsPendingSamples[0].Value)
	}
	proposalsFailedSamples, err := mc.queryForProposals(etcdProposalsFailedQuery, filters, durationInPromFormat, endTime)
	if err == nil {
		proposalsMetrics.Failed = float64(proposalsFailedSamples[0].Value)
	}

	resourceUsageMetrics := mc.queryForResourceUsage(filters.EtcdCommonFilters, durationInPromFormat, endTime)
	etcdMetrics.ResourceUsageMetrics = *resourceUsageMetrics
}

func (mc *MetricCollector) queryForProposals(query string, filters MetricFilters, durationInPromFormat string, endTime time.Time) ([]*model.Sample, error) {
	proposalsCommittedQuery := fmt.Sprintf(query, filters.EtcdCommonFilters, durationInPromFormat)
	proposalsCommittedSamples, err := mc.executor.Query(proposalsCommittedQuery, endTime)
	if err != nil {
		logQueryExecutionError(err, proposalsCommittedQuery)
		return nil, err
	}

	return proposalsCommittedSamples, nil
}

func apiServerCallInternalMetricsFromSamples(throughputSamples []*model.Sample, latencySamples []*model.Sample) (*metric.ApiCallMetrics, error) {
	m := &metric.ApiCallMetrics{MetricByKey: make(map[string]*metric.ApiCallMetric)}

	extractLabels := func(sample *model.Sample) map[string]string {
		return map[string]string{
			"resource":    string(sample.Metric["resource"]),
			"subresource": string(sample.Metric["subresource"]),
			"verb":        string(sample.Metric["verb"]),
			"scope":       string(sample.Metric["scope"]),
		}
	}
	buildKey := func(labels map[string]string) string {
		return strings.Join([]string{labels["resource"], labels["subresource"], labels["verb"], labels["scope"]}, "|")
	}

	for _, sample := range throughputSamples {
		labels := extractLabels(sample)
		key := buildKey(labels)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		throughput := rateConverter(sample.Value)
		m.SetThroughput(key, labels, quantile, throughput)
	}

	for _, sample := range latencySamples {
		labels := extractLabels(sample)
		key := buildKey(labels)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		latency := durationConverter(sample.Value)
		m.SetLatency(key, labels, quantile, latency)
	}

	return m, nil
}

func apiServerCallExternalMetricsFromSamples(throughputSamples []*model.Sample, latencySamples []*model.Sample) (*metric.ApiCallMetrics, error) {
	m := &metric.ApiCallMetrics{MetricByKey: make(map[string]*metric.ApiCallMetric)}

	extractLabels := func(sample *model.Sample) map[string]string {
		return map[string]string{
			"method": string(sample.Metric["method"]),
		}
	}
	buildKey := func(labels map[string]string) string {
		return strings.Join([]string{labels["method"]}, "|")
	}

	for _, sample := range throughputSamples {
		labels := extractLabels(sample)
		key := buildKey(labels)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		throughput := rateConverter(sample.Value)
		m.SetThroughput(key, labels, quantile, throughput)
	}

	for _, sample := range latencySamples {
		labels := extractLabels(sample)
		key := buildKey(labels)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		latency := durationConverter(sample.Value)
		m.SetLatency(key, labels, quantile, latency)
	}

	return m, nil
}

func (mc *MetricCollector) queryForResourceUsage(filters string, durationInPromFormat string, endTime time.Time) *metric.ResourceUsageMetrics {
	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(goProcessCpuQuery, q, filters, goProcessRateEvaluationRange, durationInPromFormat, goProcessRateResolution)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		cpuSamples = append(cpuSamples, samples...)
	}

	var memorySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(goProcessMemoryQuery, q, filters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		memorySamples = append(memorySamples, samples...)
	}

	var threadSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(goProcessThreadQuery, q, filters, durationInPromFormat)
		samples, err := mc.executor.Query(query, endTime)
		if err != nil {
			logQueryExecutionError(err, query)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		threadSamples = append(threadSamples, samples...)
	}

	resourceUsageMetrics, err := resourceUsageMetricsFromSamples(cpuSamples, memorySamples, threadSamples)
	if err != nil {
		log.Errorf("prometheus metrics parsing error: %v", err)
	}
	return resourceUsageMetrics
}

func resourceUsageMetricsFromSamples(cpuSamples []*model.Sample, memorySamples []*model.Sample, threadSamples []*model.Sample) (*metric.ResourceUsageMetrics, error) {
	m := &metric.ResourceUsageMetrics{}

	for _, sample := range cpuSamples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := math.Round(float64(sample.Value)*1000) / 1000
		m.CpuUsage.SetQuantile(quantile, value)
	}

	for _, sample := range memorySamples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := memoryConverter(sample.Value)
		m.MemoryUsage.SetQuantile(quantile, value)
	}

	for _, sample := range threadSamples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := math.Round(float64(sample.Value))
		m.ThreadUsage.SetQuantile(quantile, value)
	}

	return m, nil
}

func metricStatisticsFromSamples[T int | float64 | time.Duration](samples []*model.Sample, unitConverter UnitConverter[T]) (*metric.Statistics[T], error) {
	m := &metric.Statistics[T]{}

	for _, sample := range samples {
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		value := unitConverter(sample.Value)
		m.SetQuantile(quantile, value)
	}

	return m, nil
}
