package measurement

import (
	"fmt"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"math"
	"time"
	"v-bench/measurement/util"
	"v-bench/prometheus/clients"
)

const (
	queryTimeout  = 15 * time.Minute
	queryInterval = 30 * time.Second
)

// PrometheusQueryExecutor executes queries against Prometheus.
type PrometheusQueryExecutor struct {
	client clients.Client
}

// NewPrometheusQueryExecutor creates instance of PrometheusQueryExecutor.
func NewPrometheusQueryExecutor(pc clients.Client) *PrometheusQueryExecutor {
	return &PrometheusQueryExecutor{client: pc}
}

// Query executes given prometheus query at given point in time.
func (e *PrometheusQueryExecutor) Query(query string, queryTime time.Time) ([]*model.Sample, error) {
	if queryTime.IsZero() {
		return nil, fmt.Errorf("query time can't be zero")
	}

	var body []byte
	var queryErr error

	log.Debugf("Executing %q at %v", query, queryTime.Format(time.RFC3339))
	if err := wait.PollImmediate(queryInterval, queryTimeout, func() (bool, error) {
		body, queryErr = e.client.Query(query, queryTime)
		if queryErr != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		if queryErr != nil {
			resp := "(empty)"
			if body != nil {
				resp = string(body)
			}
			return nil, fmt.Errorf("query error: %v [body: %s]", queryErr, resp)
		}
		return nil, fmt.Errorf("error: %v", err)
	}

	samples, err := util.ExtractMetricSamples(body)
	if err != nil {
		return nil, fmt.Errorf("extracting error: %v", err)
	}

	var resultSamples []*model.Sample
	for _, sample := range samples {
		if !math.IsNaN(float64(sample.Value)) {
			resultSamples = append(resultSamples, sample)
		}
	}
	log.Debugf("Got %d samples", len(resultSamples))
	return resultSamples, nil
}

func (e *PrometheusQueryExecutor) Targets(params map[string]string) ([]v1.ActiveTarget, error) {
	var body []byte
	var queryErr error

	log.Debugf("Executing query for retrieving targets")

	if err := wait.PollImmediate(queryInterval, queryTimeout, func() (bool, error) {
		body, queryErr = e.client.Targets(params)
		if queryErr != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		if queryErr != nil {
			resp := "(empty)"
			if body != nil {
				resp = string(body)
			}
			return nil, fmt.Errorf("query error: %v [body: %s]", queryErr, resp)
		}
		return nil, fmt.Errorf("error: %v", err)
	}

	targets, err := util.ExtractTargets(body)
	if err != nil {
		return nil, fmt.Errorf("extracting error: %v", err)
	}

	return targets, nil
}
