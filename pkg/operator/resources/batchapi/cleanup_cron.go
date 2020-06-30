/*
Copyright 2020 Cortex Labs, Inc.

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

package batchapi

import (
	"fmt"
	"time"

	"github.com/cortexlabs/cortex/pkg/lib/debug"
	"github.com/cortexlabs/cortex/pkg/lib/sets/strset"
	"github.com/cortexlabs/cortex/pkg/operator/config"
	"github.com/cortexlabs/cortex/pkg/operator/operator"
	"github.com/cortexlabs/cortex/pkg/types/status"
	kbatch "k8s.io/api/batch/v1"
)

func CleanupJobs() error {
	queues, err := operator.ListQueues()
	if err != nil {
		fmt.Println(err.Error()) // TODO
	}

	if len(queues) < 20 {
		return nil
	}

	jobs, err := config.K8s.ListJobs(nil)
	if err != nil {
		fmt.Println(err.Error()) // TODO
	}

	// delete if enqueue liveness failed

	k8sjobMap := map[string]kbatch.Job{}
	jobIDSetK8s := strset.Set{}
	for _, job := range jobs {
		k8sjobMap[job.Labels["jobID"]] = job
		jobIDSetK8s.Add(job.Labels["jobID"])
	}

	queueURLMap := map[string]string{}
	jobIDSetQueueURL := strset.Set{}
	for _, queueURL := range queues {
		_, jobID := operator.IdentifiersFromQueueURL(queueURL)
		jobIDSetQueueURL.Add(jobID)
		queueURLMap[jobID] = queueURL
	}

	for jobID := range strset.Difference(jobIDSetK8s, jobIDSetQueueURL) {
		fmt.Println("Only job: " + jobID)
		fmt.Println(queues)
		fmt.Println("delete jobs")
		apiName := k8sjobMap[jobID].Labels["apiName"]
		_, err := operator.DoesQueueExist(apiName, jobID) // double check queue existence because newly created queues take atleast 30 seconds to be listed in operator.ListQueues()
		if err != nil {
			// TODO
		}
		// queueURL, err := operator.QueueURL(apiName, jobID)
		metrics, _ := operator.GetQueueMetrics(apiName, jobID)
		debug.Pp(metrics)
		// _, err := config.K8s.DeleteJobs(&kmeta.ListOptions{
		// 	LabelSelector: klabels.SelectorFromSet(map[string]string{"jobID": jobID}).String(),
		// })
		// if err != nil {
		// 	return err
		// }
	}

	for jobID := range strset.Difference(jobIDSetQueueURL, jobIDSetK8s) {
		fmt.Println("Only queue")
		queueURL := queueURLMap[jobID]
		apiName, jobID := operator.IdentifiersFromQueueURL(queueURL)
		debug.Pp(apiName)
		debug.Pp(jobID)

		jobSpec, err := DownloadJobSpec(apiName, jobID)
		if err != nil {
			// DeleteJob(apiName, jobID)
			fmt.Println("failed to download job spec")
			fmt.Println(err.Error())
			continue
		}
		debug.Pp(jobSpec)
		if jobSpec.Status == status.JobEnqueuing && time.Now().Sub(jobSpec.LastUpdated) > time.Second*60 {
			jobSpec.Status = status.JobFailed
			CommitToS3(*jobSpec)
			fmt.Println("stale")
			// err := DeleteJob(apiName, jobID)
			if err != nil {
				fmt.Println("here")
			}
		}

		if jobSpec.Status != status.JobEnqueuing {
			fmt.Println("status")
			// err := DeleteJob(apiName, jobID)
			// if err != nil {
			// 	fmt.Println(err.Error())
			// }
		}
	}

	for jobID := range strset.Intersection(jobIDSetQueueURL, jobIDSetK8s) {
		queueURL := queueURLMap[jobID]
		//job := k8sjobMap[jobID]
		fmt.Println(queueURL)
		apiName, jobID := operator.IdentifiersFromQueueURL(queueURL)

		jobSpec, err := DownloadJobSpec(apiName, jobID)
		if err != nil {
			fmt.Println("jobSpec")
			//DeleteJob(apiName, jobID)
			fmt.Println(err.Error())
			continue
		}
		debug.Pp(jobSpec)

		queueMetrics, err := operator.GetQueueMetricsFromURL(queueURL)
		if err != nil {
			fmt.Println("queueMetrics")
			// DeleteJob(apiName, jobID) // TODO
			fmt.Println(err.Error())
			continue
		}
		partitionMetrics, err := GetJobMetrics(jobSpec)
		if err != nil {
			fmt.Println("partitionMetrics")
			// DeleteJob(apiName, jobID) // TODO
			fmt.Println(err.Error())
			continue
			fmt.Println(partitionMetrics)
		}

		if queueMetrics.IsEmpty() {
			fmt.Println("here empty " + jobID)
			// if job.Annotations["cortex/to-delete"] == "true" {
			// 	if partitionMetrics.JobStats.Failed+partitionMetrics.JobStats.Succeeded == jobSpec.TotalPartitions {
			// 		if partitionMetrics.JobStats.Succeeded == jobSpec.TotalPartitions {
			// 			jobSpec.Status = status.JobSucceeded
			// 		} else {
			// 			jobSpec.Status = status.JobFailed
			// 		}
			// 		jobSpec.Metrics = partitionMetrics
			// 		jobSpec.QueueMetrics = queueMetrics
			// 		jobSpec.WorkerStats = &status.WorkerStats{
			// 			Active:    int(job.Status.Active),
			// 			Failed:    int(job.Status.Failed),
			// 			Succeeded: int(job.Status.Succeeded),
			// 		}
			// 		jobSpec.EndTime = pointer.Time(time.Now())
			// 		err := CommitToS3(*jobSpec)
			// 		if err != nil {
			// 			// TODO
			// 		}
			// 		DeleteJob(apiName, jobID) // TODO
			// 	}
			// } else {
			// 	if job.Annotations == nil {
			// 		job.Annotations = map[string]string{}
			// 	}
			// 	job.Annotations["cortex/to-delete"] = "true"
			// 	config.K8s.UpdateJob(&job)
			// }
		} else {
			fmt.Println("here not empty" + jobID)

			// if int(job.Status.Active) == 0 {
			// 	if job.Annotations["cortex/to-delete"] == "true" {
			// 		jobSpec.Status = status.JobIncomplete
			// 		jobSpec.Metrics = partitionMetrics
			// 		jobSpec.QueueMetrics = queueMetrics
			// 		jobSpec.WorkerStats = &status.WorkerStats{
			// 			Active:    int(job.Status.Active),
			// 			Failed:    int(job.Status.Failed),
			// 			Succeeded: int(job.Status.Succeeded),
			// 		}
			// 		jobSpec.EndTime = pointer.Time(time.Now())
			// 		err := CommitToS3(*jobSpec)
			// 		if err != nil {
			// 			// TODO
			// 		}
			// 		DeleteJob(apiName, jobID) // TODO
			// 	} else {
			// 		if job.Annotations == nil {
			// 			job.Annotations = map[string]string{}
			// 		}
			// 		job.Annotations["cortex/to-delete"] = "true"
			// 		config.K8s.UpdateJob(&job)
			// 	}
			// }
		}
	}

	return nil
}
