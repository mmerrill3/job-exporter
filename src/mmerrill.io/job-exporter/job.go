/*
Copyright 2017 Vonage All rights reserved.

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

package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	v1 "k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/client-go/tools/cache"
	"os"
)

type jobAnnotation struct {
	Kind       string
	APIVersion string
	Reference  struct {
		Kind            string
		Namespace       string
		Name            string
		UID             string
		APIVersion      string
		ResourceVersion string
	}
}

type jobGauge struct {
	name           string
	status         int64
	startTime      int64
	completionTime int64
	namespace      string
}

var (
	descJobStatus = prometheus.NewDesc(
		"kube_job_status",
		"The last run status of a job",
		[]string{"namespace", "name"}, nil,
	)
	descJobStartTime = prometheus.NewDesc(
		"kube_job_start_time",
		"The start time for a job",
		[]string{"namespace", "name"}, nil,
	)
	descJobCompletionTime = prometheus.NewDesc(
		"kube_job_completion_time",
		"The completion time for a job",
		[]string{"namespace", "name"}, nil,
	)
	jobMap = make(map[string]jobGauge)
)

type JobsLister func() ([]v1.Job, error)

func (l JobsLister) List() ([]v1.Job, error) {
	return l()
}

func RegisterJobsCollector(registry prometheus.Registerer, kubeClient kubernetes.Interface) {
	client := kubeClient.BatchV1().RESTClient()
	jlw := cache.NewListWatchFromClient(client, "jobs", api.NamespaceAll, nil)
	jinf := cache.NewSharedInformer(jlw, &v1.Job{}, resyncPeriod)

	jLister := JobsLister(func() (jobs []v1.Job, err error) {
		for _, c := range jinf.GetStore().List() {
			jobs = append(jobs, *(c.(*v1.Job)))
		}
		return jobs, nil
	})

	registry.MustRegister(&jobsCollector{store: jLister})
	go jinf.Run(context.Background().Done())
}

type jobsStore interface {
	List() (jobs []v1.Job, err error)
}

// jobsCollector collects metrics about all jobs in the cluster.
type jobsCollector struct {
	store jobsStore
}

// Describe implements the prometheus.Collector interface.
func (jc *jobsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descJobStatus
	ch <- descJobStartTime
	ch <- descJobCompletionTime
}

// Collect implements the prometheus.Collector interface.
func (jc *jobsCollector) Collect(ch chan<- prometheus.Metric) {
	jls, err := jc.store.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "listing jobs failed: %s", err)
		return
	}
	for _, j := range jls {
		jc.collectJobs(j)
	}

	addGauge := func(desc *prometheus.Desc, v float64, jobGauge *jobGauge, lv ...string) {
		lv = append([]string{jobGauge.namespace, jobGauge.name}, lv...)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
	}

	for _, jobGauge := range jobMap {
		addGauge(descJobStatus, float64(jobGauge.status), &jobGauge)
		addGauge(descJobStartTime, float64(jobGauge.startTime), &jobGauge)
		addGauge(descJobCompletionTime, float64(jobGauge.completionTime), &jobGauge)
	}
}

func (jc *jobsCollector) collectJobs(j v1.Job) {
	if nil == j.Annotations || nil == j.Status.CompletionTime {
		glog.Infof("found job that is either not done or unrecognized: %v", j.Name)
		return
	}
	status := jc.findStatus(&j)
	jobMapKey := jc.findMapKey(&j)
	if "" == jobMapKey {
		return
	}
	jobGauge, ok := jobMap[jobMapKey]
	if !ok {
		nameJSON := j.Annotations["kubernetes.io/created-by"]
		jobAnnotation, _ := jc.decodeJSON([]byte(nameJSON))
		jobGauge.name = jobAnnotation.Reference.Name
		glog.Infof("Found gauge name %s", jobGauge.name)
		jobGauge.completionTime = j.Status.CompletionTime.Unix()
		jobGauge.startTime = j.Status.StartTime.Unix()
		jobGauge.status = status
		jobGauge.namespace = j.Namespace
		jobMap[jobMapKey] = jobGauge
	} else {
		if jobGauge.completionTime <= j.Status.CompletionTime.Unix() {
			jobGauge.completionTime = j.Status.CompletionTime.Unix()
			jobGauge.status = status
			jobGauge.startTime = j.Status.StartTime.Unix()
			jobMap[jobMapKey] = jobGauge
		}
	}
}

func (jc *jobsCollector) decodeJSON(bytes []byte) (*jobAnnotation, error) {
	var annotation jobAnnotation
	err := json.Unmarshal(bytes, &annotation)
	if err != nil {
		glog.Fatalf("Cannot parse annotation %s", err)
	}
	return &annotation, nil
}

func (jc *jobsCollector) findStatus(job *v1.Job) int64 {
	if job.Status.Active > 0 {
		return 2
	} else if job.Status.Failed > 0 {
		return 1
	}
	return 0
}

func (jb *jobsCollector) findMapKey(job *v1.Job) string {
	if nil == job.Annotations {
		return ""
	}
	return job.Annotations["name"] + job.Namespace
}
