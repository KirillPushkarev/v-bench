package reporting

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"v-bench/config"
	"v-bench/measurement"
)

type JsonReporter struct{}

type ReportModel struct {
	Metrics struct {
		HostCluster     *measurement.Metrics `json:"host_cluster"`
		VirtualClusters *measurement.Metrics `json:"virtual_clusters"`
	} `json:"metrics"`
}

func (*JsonReporter) Report(benchmarkConfig *config.TestConfig, outputPath string, hostMeasurementContext *measurement.Context, virtualMeasurementContext *measurement.Context) {
	reportModel := ReportModel{Metrics: struct {
		HostCluster     *measurement.Metrics `json:"host_cluster"`
		VirtualClusters *measurement.Metrics `json:"virtual_clusters"`
	}{HostCluster: &hostMeasurementContext.Metrics}}
	if benchmarkConfig.ClusterType == config.ClusterTypeVirtual {
		reportModel.Metrics.VirtualClusters = &virtualMeasurementContext.Metrics
	}

	content, err := json.Marshal(reportModel)
	if err != nil {
		log.Fatal(err)
	}
	reportPath := filepath.Join(outputPath, "report.json")
	err = os.WriteFile(reportPath, content, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Global-scoped metrics saved to file: %v", reportPath)
}
