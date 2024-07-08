package main

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

type ResourceChecker interface {
	Check(ctx context.Context, clientset *kubernetes.Clientset, namespace, crdGroup, crdVersion, crdPlural, labelSelector, annotationSelector string, unhealthyChildren *[]UnhealthyChild, overallStatus *string) error
}

type UnhealthyChild struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"issue,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
