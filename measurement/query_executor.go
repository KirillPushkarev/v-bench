package measurement

import (
	"github.com/prometheus/common/model"
	"time"
)

// QueryExecutor is an interface for querying Prometheus server.
type QueryExecutor interface {
	Query(query string, queryTime time.Time) ([]*model.Sample, error)
}
