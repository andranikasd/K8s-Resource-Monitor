package main

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PodChecker struct{}

func (pc PodChecker) Check(ctx context.Context, clientset *kubernetes.Clientset, namespace, crdGroup, crdVersion, crdPlural, labelSelector, annotationSelector string, unhealthyChildren *[]UnhealthyChild, overallStatus *string) error {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error listing Pods: %v", err)
	}

	for _, pod := range pods.Items {
		if !matchAnnotations(pod.Annotations, annotationSelector) {
			continue
		}
		log.Printf("[INFO] Pod status: name=%s, phase=%s", pod.Name, pod.Status.Phase)

		switch pod.Status.Phase {
		case corev1.PodRunning:
			if isPodHealthy(pod) {
				continue
			} else {
				*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
					Kind:    "Pod",
					Name:    pod.Name,
					Status:  string(pod.Status.Phase),
					Message: fmt.Sprintf("Pod %s is in %s state but not all containers are ready", pod.Name, pod.Status.Phase),
					Reason:  "NotAllContainersReady",
				})
				if *overallStatus != "failed" {
					*overallStatus = "deploying"
				}
			}
		case corev1.PodPending:
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "Pod",
				Name:    pod.Name,
				Status:  string(pod.Status.Phase),
				Message: fmt.Sprintf("Pod %s is in Pending state", pod.Name),
				Reason:  "Pending",
			})
			if *overallStatus != "failed" {
				*overallStatus = "deploying"
			}
		case corev1.PodFailed, corev1.PodUnknown:
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "Pod",
				Name:    pod.Name,
				Status:  string(pod.Status.Phase),
				Message: fmt.Sprintf("Pod %s is in %s state", pod.Name, pod.Status.Phase),
				Reason:  getPodFailureReason(pod),
			})
			*overallStatus = "failed"
		case corev1.PodSucceeded:
			continue
		default:
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "Pod",
				Name:    pod.Name,
				Status:  string(pod.Status.Phase),
				Message: fmt.Sprintf("Pod %s is in an unexpected state: %s", pod.Name, pod.Status.Phase),
				Reason:  "Unknown",
			})
			if *overallStatus != "failed" {
				*overallStatus = "deploying"
			}
		}
	}

	return nil
}

func isPodHealthy(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			return false
		}
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	return true
}

func getPodFailureReason(pod corev1.Pod) string {
	if len(pod.Status.ContainerStatuses) > 0 {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				return containerStatus.State.Waiting.Reason
			}
		}
	}
	return "Unknown"
}
