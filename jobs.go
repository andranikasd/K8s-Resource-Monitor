package main

import (
	"context"
	"fmt"
	"log"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type JobChecker struct{}

func (jc JobChecker) Check(ctx context.Context, clientset *kubernetes.Clientset, namespace, crdGroup, crdVersion, crdPlural, labelSelector, annotationSelector string, unhealthyChildren *[]UnhealthyChild, overallStatus *string) error {
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	jobFailureThreshold := 3
	if err != nil {
		return fmt.Errorf("error listing Jobs: %v", err)
	}

	for _, job := range jobs.Items {
		if !matchAnnotations(job.Annotations, annotationSelector) {
			continue
		}
		log.Printf("[INFO] Job status: name=%s, succeeded=%d, failed=%d", job.Name, job.Status.Succeeded, job.Status.Failed)

		if job.Status.Failed >= int32(jobFailureThreshold) {
			log.Printf("[INFO] Job status: name=%s, succeeded=%d, failed=%d", job.Name, job.Status.Succeeded, job.Status.Failed)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "Job",
				Name:    job.Name,
				Status:  "Failed",
				Message: fmt.Sprintf("Job %s has failed %d times", job.Name, job.Status.Failed),
				Reason:  getJobFailureReason(job),
			})
			*overallStatus = "failed"
		} else if job.Status.Succeeded == 0 {
			log.Printf("[INFO] Job status: name=%s, succeeded=%d, failed=%d", job.Name, job.Status.Succeeded, job.Status.Failed)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "Job",
				Name:    job.Name,
				Status:  "Pending",
				Message: fmt.Sprintf("Job %s is in Pending state", job.Name),
				Reason:  getJobFailureReason(job),
			})
			if *overallStatus != "failed" {
				*overallStatus = "deploying"
			}
		}
	}

	return nil
}

func getJobFailureReason(job batchv1.Job) string {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed {
			return condition.Reason
		}
	}
	return "Unknown"
}
