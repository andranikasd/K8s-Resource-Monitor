package main

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PVChecker struct{}

func (pvChecker PVChecker) Check(ctx context.Context, clientset *kubernetes.Clientset, namespace, crdGroup, crdVersion, crdPlural, labelSelector, annotationSelector string, unhealthyChildren *[]UnhealthyChild, overallStatus *string) error {
	pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error listing PVs: %v", err)
	}

	for _, pv := range pvs.Items {
		if !matchAnnotations(pv.Annotations, annotationSelector) {
			continue
		}
		log.Printf("[INFO] PV status: name=%s, phase=%s", pv.Name, pv.Status.Phase)

		switch pv.Status.Phase {
		case corev1.VolumeBound:
			log.Printf("[INFO] PV %s is Healthy", pv.Name)
			continue
		case corev1.VolumeAvailable:
			log.Printf("[INFO] PV %s is Available", pv.Name)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PV",
				Name:    pv.Name,
				Status:  string(pv.Status.Phase),
				Message: fmt.Sprintf("PV %s is Available", pv.Name),
				Reason:  "Available",
			})
			if *overallStatus != "failed" {
				*overallStatus = "ready"
			}
		case corev1.VolumeFailed:
			log.Printf("[INFO] PV %s is in Failed state", pv.Name)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PV",
				Name:    pv.Name,
				Status:  string(pv.Status.Phase),
				Message: fmt.Sprintf("PV %s is in Failed state", pv.Name),
				Reason:  "Failed",
			})
			*overallStatus = "failed"
		case corev1.VolumeReleased:
			log.Printf("[INFO] PV %s is in Released state", pv.Name)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PV",
				Name:    pv.Name,
				Status:  string(pv.Status.Phase),
				Message: fmt.Sprintf("PV %s is in Released state", pv.Name),
				Reason:  "Released",
			})
			if *overallStatus != "failed" {
				*overallStatus = "deploying"
			}
		default:
			log.Printf("[INFO] PV %s is in an unexpected state: %s", pv.Name, pv.Status.Phase)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PV",
				Name:    pv.Name,
				Status:  string(pv.Status.Phase),
				Message: fmt.Sprintf("PV %s is in an unexpected state: %s", pv.Name, pv.Status.Phase),
				Reason:  "Unknown",
			})
			if *overallStatus != "failed" {
				*overallStatus = "failed"
			}
		}
	}

	return nil
}
