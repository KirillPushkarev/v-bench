package k8s

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"text/template"
	"v-bench/config"
)

func ApplyManifest(clusterConfig *config.ClusterConfig, manifestName string, manifest string, data any) error {
	log.Debugf("Cluster %v; applying manifest: %s", clusterConfig.Name, manifestName)

	t, err := template.New(manifestName).Parse(manifest)
	if err != nil {
		return err
	}
	buffer := new(bytes.Buffer)
	err = t.Execute(buffer, data)
	if err != nil {
		return err
	}

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = buffer
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("Cluster %v; error on applying manifest: %e, command result: %v", clusterConfig.Name, err, string(stdoutStderr))
	}

	log.Debugf("Cluster %v; finished applying manifest: %s", clusterConfig.Name, manifestName)

	return nil
}
