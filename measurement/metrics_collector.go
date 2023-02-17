package measurement

import (
	"fmt"
	"github.com/prometheus/common/model"
	"strconv"
	"time"
	"v-bench/config"
	"v-bench/k8s"
	measurementutil "v-bench/measurement/util"
	"v-bench/prometheus/clients"
)

const (
	numK8sClients = 1

	// apiCallLatencyQuery measures 99th percentile of API call latency over given period of time
	// apiCallLatencyQuery: placeholders should be replaced with (1) quantile (2) apiCallLatencyFilters and (3) query window size.
	apiCallLatencyQuery   = "histogram_quantile(%.2f, sum(rate(apiserver_request_duration_seconds_bucket{%v}[%v])) by (resource,  subresource, verb, scope, le))"
	apiCallLatencyFilters = `verb!="WATCH", subresource!~"log|exec|portforward|attach|proxy"`
	// TODO: Fix, calculating quantile over counter is meaningless
	apiServerCpuQuery             = "quantile_over_time(%.2f, process_cpu_seconds_total{%v}[%v])"
	apiServerMemoryQuery          = "quantile_over_time(%.2f, process_resident_memory_bytes{%v}[%v])"
	apiServerThreadQuery          = "quantile_over_time(%.2f, go_goroutines{%v}[%v])"
	apiServerResourceUsageFilters = "job=\"apiserver\""
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
	measurementDuration := endTime.Sub(context.StartTime)
	promDuration := measurementutil.ToPrometheusTime(measurementDuration)

	collectApiServerLatencyMetrics(context, executor, endTime, promDuration)
	collectApiServerResourceMetrics(context, executor, endTime, promDuration)
	//collectControllerManagerMetrics(promDuration, executor, endTime, err, context)
	//collectSchedulerMetrics(promDuration, executor, endTime, err, context)
	//collectEtcdMetrics(promDuration, executor, endTime, err, context)
	//collectOverallControlPlaneMetrics(promDuration, executor, endTime, err, context)
}

func collectApiServerLatencyMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	var latencySamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiCallLatencyQuery, q, apiCallLatencyFilters, durationInPromFormat)
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
	context.SetApiCallMetrics(apiCallMetrics)
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

func collectApiServerResourceMetrics(context *Context, executor *PrometheusQueryExecutor, endTime time.Time, durationInPromFormat string) {
	var cpuSamples []*model.Sample
	for _, q := range quantiles {
		query := fmt.Sprintf(apiServerCpuQuery, q, apiServerResourceUsageFilters, durationInPromFormat)
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
	context.SetApiServerResourceUsageMetrics(resourceUsageMetrics)
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
