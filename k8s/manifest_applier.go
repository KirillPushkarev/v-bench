package k8s

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"os/exec"
	"text/template"
)

const (
	RootCluster  = "root"
	MethodCreate = "create"
	MethodApply  = "apply"
)

func ApplyManifestFromString(clusterName string, kubeconfigPath string, manifestName string, manifest string, data any, method string) error {
	log.Debugf("Cluster %v; applying manifest: %s", clusterName, manifestName)

	t, err := template.New(manifestName).Parse(manifest)
	if err != nil {
		return err
	}
	buffer := new(bytes.Buffer)
	err = t.Execute(buffer, data)
	if err != nil {
		return err
	}

	cmd := exec.Command("kubectl", method, "-f", "-")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", kubeconfigPath))
	cmd.Stdin = buffer
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Cluster %v; error on applying manifest: %v, command result: %v", clusterName, err, string(stdoutStderr))
	}

	log.Debugf("Cluster %v; finished applying manifest: %s", clusterName, manifestName)

	return nil
}

func ApplyManifestFromEmbeddedFile(clusterName string, kubeconfigPath string, manifestFs fs.FS, manifestPath string, data any, method string) error {
	log.Debugf("Cluster %v; applying manifest: %s", clusterName, manifestPath)

	t, err := template.ParseFS(manifestFs, manifestPath)
	if err != nil {
		return err
	}
	buffer := new(bytes.Buffer)
	err = t.Execute(buffer, data)
	if err != nil {
		return err
	}

	cmd := exec.Command("kubectl", method, "-f", "-")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", kubeconfigPath))
	cmd.Stdin = buffer
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Cluster %v; error on applying manifest: %v, command result: %v", clusterName, err, string(stdoutStderr))
	}

	log.Debugf("Cluster %v; finished applying manifest: %s", clusterName, manifestPath)

	return nil
}
