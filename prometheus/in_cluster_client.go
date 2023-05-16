package prometheus

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	"v-bench/config"

	clientset "k8s.io/client-go/kubernetes"
)

// inClusterClient talks to the Prometheus instance deployed in the test cluster.
type inClusterClient struct {
	prometheusConnType config.PrometheusConnType
	client             clientset.Interface
}

func NewInClusterClient(prometheusConnType config.PrometheusConnType, client clientset.Interface) Client {
	return &inClusterClient{prometheusConnType: prometheusConnType, client: client}
}

func (client *inClusterClient) Query(query string, queryTime time.Time) ([]byte, error) {
	paramsMap := map[string]string{
		"query": query,
		"time":  queryTime.Format(time.RFC3339),
	}
	baseURL := "http://localhost:9090"
	path := "api/v1/query"

	return client.makeRequest(baseURL, path, paramsMap)
}

func (client *inClusterClient) Targets(state string) ([]byte, error) {
	paramsMap := map[string]string{
		"state": state,
	}
	baseURL := "http://localhost:9090"
	path := "api/v1/targets"

	return client.makeRequest(baseURL, path, paramsMap)
}

func (client *inClusterClient) makeRequest(baseURL string, path string, paramsMap map[string]string) ([]byte, error) {
	if client.prometheusConnType == config.PrometheusConnTypeDirect {
		return client.makeDirectRequest(baseURL, path, paramsMap)
	}

	return client.makeProxyRequest(path, paramsMap)
}

func (client *inClusterClient) makeDirectRequest(baseURL string, path string, paramsMap map[string]string) ([]byte, error) {
	params := url.Values{}
	for k, v := range paramsMap {
		params.Add(k, v)
	}

	u, _ := url.ParseRequestURI(baseURL)
	u.Path = path
	u.RawQuery = params.Encode()
	requestURL := fmt.Sprintf("%v", u)

	res, err := http.Get(requestURL)
	if err != nil {
		return nil, err
	}
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return resBody, err
}

func (client *inClusterClient) makeProxyRequest(path string, paramsMap map[string]string) ([]byte, error) {
	return client.client.CoreV1().
		Services("monitoring").
		ProxyGet("http", "prometheus-k8s", "9090", path, paramsMap).
		DoRaw(context.TODO())
}
