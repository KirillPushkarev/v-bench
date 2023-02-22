package measurement

type MetricStatistics[T int | float64] struct {
	Perc50 T `json:"Perc50"`
	Perc90 T `json:"Perc90"`
	Perc99 T `json:"Perc99"`
}

func (metric *MetricStatistics[T]) SetQuantile(quantile float64, value T) {
	switch quantile {
	case 0.5:
		metric.Perc50 = value
	case 0.9:
		metric.Perc90 = value
	case 0.99:
		metric.Perc99 = value
	}
}
