/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"encoding/json"
	"fmt"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
)

// ExtractMetricSamples unpacks metric blob into prometheus model structures.
func ExtractMetricSamples(response []byte) ([]*model.Sample, error) {
	var pqr promQueryResponse
	if err := json.Unmarshal(response, &pqr); err != nil {
		return nil, err
	}
	if pqr.Status != "success" {
		return nil, fmt.Errorf("non-success response status: %v", pqr.Status)
	}
	vector, ok := pqr.Data.v.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("incorrect response type: %v", pqr.Data.v.Type())
	}
	return vector, nil
}

// promQueryResponse stores the response from the Prometheus server.
// This struct follows the format described in the Prometheus documentation:
// https://prometheus.io/docs/prometheus/latest/querying/api/#format-overview.
type promQueryResponse struct {
	Status    string           `json:"status"`
	Data      promResponseData `json:"data"`
	ErrorType string           `json:"errorType"`
	Error     string           `json:"error"`
	Warnings  []string         `json:"warnings"`
}

type promResponseData struct {
	v model.Value
}

// UnmarshalJSON unmarshals json into promResponseData structure.
func (qr *promResponseData) UnmarshalJSON(b []byte) error {
	v := struct {
		Type   model.ValueType `json:"resultType"`
		Result json.RawMessage `json:"result"`
	}{}

	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case model.ValScalar:
		var sv model.Scalar
		err = json.Unmarshal(v.Result, &sv)
		qr.v = &sv
	case model.ValVector:
		var vv model.Vector
		err = json.Unmarshal(v.Result, &vv)
		qr.v = vv
	case model.ValMatrix:
		var mv model.Matrix
		err = json.Unmarshal(v.Result, &mv)
		qr.v = mv
	default:
		err = fmt.Errorf("unexpected value type %q", v.Type)
	}
	return err
}

// ToPrometheusTime returns prometheus string representation of given time.
func ToPrometheusTime(t time.Duration) string {
	if t < time.Minute {
		return fmt.Sprintf("%ds", int64(t)/int64(time.Second))
	}
	return fmt.Sprintf("%dm", int64(t)/int64(time.Minute))
}

type promTargetsResponse struct {
	Status string                  `json:"status"`
	Data   promTargetsResponseData `json:"data"`
}

type promTargetsResponseData struct {
	ActiveTargets  []v1.ActiveTarget `json:"activeTargets"`
	DroppedTargets []v1.ActiveTarget `json:"droppedTargets"`
}

func ExtractTargets(response []byte) ([]v1.ActiveTarget, error) {
	var ptr promTargetsResponse
	if err := json.Unmarshal(response, &ptr); err != nil {
		return nil, err
	}
	if ptr.Status != "success" {
		return nil, fmt.Errorf("non-success response status: %v", ptr.Status)
	}

	activeTargets := ptr.Data.ActiveTargets
	return activeTargets, nil
}
