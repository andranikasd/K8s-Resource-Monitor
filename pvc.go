package main

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PVCChecker struct{}

func (pvcChecker PVCChecker) Check(ctx context.Context, clientset *kubernetes.Clientset, namespace, crdGroup, crdVersion, crdPlural, labelSelector, annotationSelector string, unhealthyChildren *[]UnhealthyChild, overallStatus *string) error {
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error listing PVCs: %v", err)
	}

	for _, pvc := range pvcs.Items {
		if !matchAnnotations(pvc.Annotations, annotationSelector) {
			continue
		}
		log.Printf("[INFO] PVC status: name=%s, phase=%s", pvc.Name, pvc.Status.Phase)

		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			log.Printf("[INFO] PVC %s is Healthy", pvc.Name)
			continue
		case corev1.ClaimPending:
			log.Printf("[INFO] PVC %s is Pending", pvc.Name)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PVC",
				Name:    pvc.Name,
				Status:  string(pvc.Status.Phase),
				Message: fmt.Sprintf("PVC %s is Pending", pvc.Name),
				Reason:  "Pending",
			})
			if *overallStatus != "failed" {
				*overallStatus = "deploying"
			}
		case corev1.ClaimLost:
			log.Printf("[INFO] PVC %s is in Lost state", pvc.Name)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PVC",
				Name:    pvc.Name,
				Status:  string(pvc.Status.Phase),
				Message: fmt.Sprintf("PVC %s is in Lost state", pvc.Name),
				Reason:  "Lost",
			})
			*overallStatus = "failed"
		default:
			log.Printf("[INFO] PVC %s is in an unexpected state: %s", pvc.Name, pvc.Status.Phase)
			*unhealthyChildren = append(*unhealthyChildren, UnhealthyChild{
				Kind:    "PVC",
				Name:    pvc.Name,
				Status:  string(pvc.Status.Phase),
				Message: fmt.Sprintf("PVC %s is in an unexpected state: %s", pvc.Name, pvc.Status.Phase),
				Reason:  "Unknown",
			})
			*overallStatus = "failed"
		}
	}

	return nil
}
