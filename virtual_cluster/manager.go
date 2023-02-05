package virtual_cluster

import "v-bench/common"

type VirtualClusterManager interface {
	Create(benchmarkConfig common.TestConfig)
	Delete(benchmarkConfig common.TestConfig)
}
