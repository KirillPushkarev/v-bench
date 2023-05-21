package measurement

import (
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
)

// QueryExecutor is an interface for querying Prometheus server.
type QueryExecutor interface {
	Query(query string, queryTime time.Time) ([]*model.Sample, error)
	Targets(state string) ([]v1.ActiveTarget, error)
}
