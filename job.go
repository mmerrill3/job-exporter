/*
Copyright 2016 Mike Merrill All rights reserved.

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
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/client-go/tools/cache"
)

var (
	descJobStatus = prometheus.NewDesc(
		"kube_job_status",
		"The last run status of a job",
		[]string{"namespace", "name"}, nil,
	)
	descJobStartTime = prometheus.NewDesc(
		"kube_job_status_time",
		"The start time for a job",
		[]string{"namespace", "name"}, nil,
	)
	descJobCompletionTime = prometheus.NewDesc(
		"kube_job_completion_time",
		"The completion time for a job",
		[]string{"namespace", "name"}, nil,
	)
)

type JobsLister func() ([]v1.Job, error)

func (l JobsLister) List() ([]v1.Job, error) {
	return l()
}

func RegisterJobsCollector(registry prometheus.Registerer, kubeClient kubernetes.Interface) {
	client := kubeClient.Extensions().RESTClient()
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
		glog.Errorf("listing jobs failed: %s", err)
		return
	}
	for _, j := range jls {
		jc.collectJobs(ch, j)
	}
}

func (jc *jobsCollector) collectJobs(ch chan<- prometheus.Metric, j v1.Job) {
	addGauge := func(desc *prometheus.Desc, v float64, lv ...string) {
		lv = append([]string{j.Namespace, j.Name}, lv...)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
	}
	if j.Status.Active > 0 {
		addGauge(descJobStatus, 2)
	} else if j.Status.Failed > 0 {
		addGauge(descJobStatus, 0)
	} else {
		addGauge(descJobStatus, 1)
	}
	addGauge(descJobStartTime, float64(j.Status.StartTime.Unix()))
	addGauge(descJobCompletionTime, float64(j.Status.CompletionTime.Unix()))
}
