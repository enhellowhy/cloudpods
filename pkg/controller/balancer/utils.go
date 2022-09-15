// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package balancer

import (
	"crypto/sha256"
	"fmt"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"

	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/monitor"
	mq "yunion.io/x/onecloud/pkg/monitor/metricquery"
)

type Metric struct {
	Id     string
	Values map[string]float64
}

type Metrics struct {
	metrics []*Metric
	indexes map[string]*Metric
}

func NewMetrics(ms []*Metric) *Metrics {
	h := &Metrics{
		metrics: ms,
		indexes: make(map[string]*Metric),
	}
	for _, m := range ms {
		h.indexes[m.Id] = m
	}
	return h
}

func (hs Metrics) JSONString() string {
	return jsonutils.Marshal(hs.metrics).String()
}

func (hs Metrics) Get(id string) *Metric {
	return hs.indexes[id]
}

type TsdbQuery struct {
	Database    string
	Measurement string
	Fields      []string
}

func InfluxdbQuery(
	session *mcclient.ClientSession,
	idKey string,
	id string,
	params *TsdbQuery) (*Metrics, error) {

	inputQuery := new(api.MetricInputQuery)
	inputQuery.To = "now"
	inputQuery.From = "10m"
	inputQuery.Scope = "system"
	query := new(api.AlertQuery)

	qp1 := api.MetricQueryPart{Type: "field", Params: params.Fields}
	qp2 := api.MetricQueryPart{Type: "mean", Params: []string{}}

	tags := make([]api.MetricQueryTag, 0)
	tag1 := api.MetricQueryTag{Key: idKey, Value: id, Operator: "="}
	tags = append(tags, tag1)
	if params.Measurement == "agent_cpu" {
		tag2 := api.MetricQueryTag{Key: "cpu", Value: "cpu-total", Operator: "="}
		tags = append(tags, tag2)
	}

	query.Model = api.MetricQuery{
		Selects: []api.MetricQuerySelect{[]api.MetricQueryPart{
			qp1,
			qp2,
		}},
		Tags:        tags,
		Measurement: params.Measurement,
		Alias:       params.Fields[0],
	}
	inputQuery.MetricQuery = []*api.AlertQuery{query}
	data, _ := jsonutils.Marshal(inputQuery).(*jsonutils.JSONDict)
	inputQuery.Signature = fmt.Sprintf("%x", sha256.Sum256([]byte(data.String())))
	data1, _ := jsonutils.Marshal(inputQuery).(*jsonutils.JSONDict)

	obj, err := modules.UnifiedMonitorManager.PerformAction(session, "query", "", data1)
	if err != nil {
		log.Errorf("query metrics err in balancer %v", err)
		return nil, err
	}
	metrics := new(mq.Metrics)
	err = obj.Unmarshal(metrics)
	if err != nil {
		log.Errorf("Unmarshal metrics err in balancer %v", err)
		return nil, err
	}

	if metrics.Series == nil {
		log.Errorln("metrics.Series is nil")
		return nil, fmt.Errorf("metrics.Series is nil")
	}
	ms := make([]*Metric, len(metrics.Series))
	for i, s := range metrics.Series {
		m := &Metric{
			Id:     s.Tags[idKey],
			Values: make(map[string]float64),
		}
		for j, f := range params.Fields {
			//m.Values[f] = *(s.Points[0][j].(*float64))
			m.Values[f] = s.Points[0][j].(float64)
		}
		ms[i] = m
	}
	return NewMetrics(ms), nil
}
