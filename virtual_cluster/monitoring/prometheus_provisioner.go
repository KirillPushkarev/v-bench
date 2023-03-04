package monitoring

type PrometheusProvisioner struct {
}

func NewPrometheusProvisioner() *PrometheusProvisioner {
	return &PrometheusProvisioner{}
}

func (receiver PrometheusProvisioner) Provision(kubeConfigPath string) {

}
