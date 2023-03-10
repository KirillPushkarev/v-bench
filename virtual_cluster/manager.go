package virtual_cluster

import "v-bench/config"

type VirtualClusterManager interface {
	Create(benchmarkConfig *config.TestConfig)
	Delete(benchmarkConfig *config.TestConfig)
}
