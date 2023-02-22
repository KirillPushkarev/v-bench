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

	// apiServerLatencyQuery measures 99th percentile of API call latency over given period of time
	// apiServerLatencyQuery: placeholders should be replaced with (1) quantile (2) apiServerCommonFilters and (3) query window size.
	apiServerLatencyQuery           = "histogram_quantile(%.2f, sum(rate(apiserver_request_duration_seconds_bucket{%v}[%v])) by (resource,  subresource, verb, scope, le))"
	apiServerCommonFilters          = `verb!="WATCH", subresource!~"log|exec|portforward|attach|proxy"`
	apiServerCpuQuery               = "quantile_over_time(%.2f, rate(process_cpu_seconds_total{%v}[%v])[%v:%v])"
	apiServerCpuRateEvaluationRange = "1m"
	apiServerCpuResolution          = "1m"
	apiServerMemoryQuery            = "quantile_over_time(%.2f, process_resident_memory_bytes{%v}[%v])"
	apiServerThreadQuery            = "quantile_over_time(%.2f, go_goroutines{%v}[%v])"
	apiServerResourceUsageFilters   = `job="apiserver"`

	etcdLeaderElectionsQuery    = "increase(etcd_server_leader_changes_seen_total{%v}[%v])"
	etcdDbSizeQuery             = "quantile_over_time(%.2f, etcd_mvcc_db_total_size_in_bytes{%v}[%v])"
	etcdWalSyncQuery            = "histogram_quantile(%.2f, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket{%v}[%v])) by (le))"
	etcdBackendCommitSyncQuery  = "histogram_quantile(%.2f, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket{%v}[%v])) by (le))"
	etcdProposalsCommittedQuery = "sum(rate(etcd_server_proposals_committed_total{%v}[%v])"
	etcdProposalsAppliedQuery   = "sum(rate(etcd_server_proposals_applied_total{%v}[%v])"
	etcdProposalsPendingQuery   = "sum(quantile_over_time(0.5, etcd_server_proposals_pending{%v}[%v]))"
	etcdProposalsFailedQuery    = "sum(rate(etcd_server_proposals_failed_total{%v}[%v])"
	etcdCommonFilters           = `job="etcd"`
	etcdCpuQuery                = "quantile_over_time(%.2f, rate(process_cpu_seconds_total{%v}[%v])[%v:%v])"
	etcdCpuRateEvaluationRange  = "1m"
	etcdCpuResolution           = "1m"
	etcdMemoryQuery             = "quantile_over_time(%.2f, process_resident_memory_bytes{%v}[%v])"
	etcdThreadQuery             = "quantile_over_time(%.2f, go_goroutines{%v}[%v])"
	etcdResourceUsageFilters    = `job="etcd"`
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

	collectApiServerLatencyMetrics(context, executor, endTime, durationInPromFormat)
	collectApiServerResourceUsageMetrics(context, executor, endTime, durationInPromFormat)
	//collectControllerManagerMetrics(durationInPromFormat, executor, endTime, err, context)
	//collectSchedulerMetrics(durationInPromFormat, executor, endTime, err, context)
	collectEtcdMetrics(context, executor, endTime, durationInPromFormat)
	//collectOverallControlPlaneMetrics(durationInPromFormat, executor, endTime, err, context)
}

func collectApiServerLatencyMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	var latencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerLatencyQuery, q, apiServerCommonFilters, durationInPromFormat)
		samples, err := executor.Query(query, endTime)
		if err != nil {
			fmt.Printf("prometheus query execution error: %v", err)
		}
		for _, sample := range samples {
			sample.Metric["quantile"] = model.LabelValue(fmt.Sprintf("%.2f", q))
		}
		latencySamples = append(latencySamples, samples...)
	}

	apiCallMetrics, err := apiCallMetricsFromSamples(latencySamples)
	if err != nil {
		fmt.Printf("prometheus metrics parsing error: %v", err)
	}
	context.Metrics.ApiServerMetrics.ApiCallMetrics = *apiCallMetrics
}

func apiCallMetricsFromSamples(latencySamples []*model.Sample) (*ApiCallMetrics, error) {
	extractLabelValues := func(sample *model.Sample) (string, string, string, string) {
		return string(sample.Metric["resource"]), string(sample.Metric["subresource"]), string(sample.Metric["verb"]), string(sample.Metric["scope"])
	}

	m := &ApiCallMetrics{Metrics: make(map[string]*ApiCallMetric)}

	for _, sample := range latencySamples {
		resource, subresource, verb, scope := extractLabelValues(sample)
		quantile, err := strconv.ParseFloat(string(sample.Metric["quantile"]), 64)
		if err != nil {
			return nil, err
		}

		latency := time.Duration(float64(sample.Value) * float64(time.Second))
		m.SetLatency(resource, subresource, verb, scope, quantile, latency)
	}

	return m, nil
}

func collectApiServerResourceUsageMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerCpuQuery, q, apiServerResourceUsageFilters, apiServerCpuRateEvaluationRange, durationInPromFormat, apiServerCpuResolution)
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
	context.Metrics.ApiServerMetrics.ResourceUsageMetrics = *resourceUsageMetrics
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
		query := fmt.Sprintf(etcdDbSizeQuery, q, etcdResourceUsageFilters, durationInPromFormat)
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

	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(etcdCpuQuery, q, etcdResourceUsageFilters, etcdCpuRateEvaluationRange, durationInPromFormat, etcdCpuResolution)
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
		query := fmt.Sprintf(etcdMemoryQuery, q, etcdResourceUsageFilters, durationInPromFormat)
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
		query := fmt.Sprintf(etcdThreadQuery, q, etcdResourceUsageFilters, durationInPromFormat)
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
