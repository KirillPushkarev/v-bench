package reporting

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
	"v-bench/config"
)

type LifeCycleTestJsonReporter struct{}

type Metrics struct {
	CreateLatency []time.Duration `json:"create_latency"`
	DeleteLatency []time.Duration `json:"delete_latency"`
}

type LifeCycleReportModel struct {
	Metrics Metrics `json:"metrics"`
}

func (*LifeCycleTestJsonReporter) Report(benchmarkConfig *config.LifecycleTestConfig, outputPath string, createDurations []time.Duration, deleteDurations []time.Duration) {
	reportModel := LifeCycleReportModel{
		Metrics{CreateLatency: createDurations, DeleteLatency: deleteDurations},
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

	log.Infof("Metrics saved to file: %v", reportPath)
}
