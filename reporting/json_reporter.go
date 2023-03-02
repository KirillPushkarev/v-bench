package reporting

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"v-bench/measurement"
)

type JsonReporter struct{}

func (*JsonReporter) Report(outputPath string, measurementContext *measurement.Context) {
	content, err := json.Marshal(struct {
		Metrics measurement.Metrics `json:"metrics"`
	}{Metrics: measurementContext.Metrics})
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
