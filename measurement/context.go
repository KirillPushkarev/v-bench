package measurement

import "time"

type Context struct {
	StartTime        time.Time
	ApiServerMetrics ApiServerMetrics
}

type ApiServerMetrics struct {
	ApiCallMetrics       ApiCallMetrics
	ResourceUsageMetrics ResourceUsageMetrics
}

func NewContext(startTime time.Time) *Context {
	return &Context{StartTime: startTime}
}

func (c *Context) SetApiCallMetrics(metrics *ApiCallMetrics) {
	c.ApiServerMetrics.ApiCallMetrics = *metrics
}

func (c *Context) SetApiServerResourceUsageMetrics(metrics *ResourceUsageMetrics) {
	c.ApiServerMetrics.ResourceUsageMetrics = *metrics
}
