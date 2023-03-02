package metric

import "time"

type Statistics[T int | float64 | time.Duration] struct {
	Perc50 T `json:"p50"`
	Perc90 T `json:"p90"`
	Perc99 T `json:"p99"`
}

func (metric *Statistics[T]) SetQuantile(quantile float64, value T) {
	switch quantile {
	case 0.5:
		metric.Perc50 = value
	case 0.9:
		metric.Perc90 = value
	case 0.99:
		metric.Perc99 = value
	}
}
