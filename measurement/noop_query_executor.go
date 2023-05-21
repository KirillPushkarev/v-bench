package measurement

import (
	"errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
)

type NoOpQueryExecutor struct {
}

func NewNoOpQueryExecutor() QueryExecutor {
	return &NoOpQueryExecutor{}
}

func (e *NoOpQueryExecutor) Query(query string, queryTime time.Time) ([]*model.Sample, error) {
	return nil, errors.New("should not be called")
}

func (e *NoOpQueryExecutor) Targets(state string) ([]v1.ActiveTarget, error) {
	return nil, errors.New("should not be called")
}
