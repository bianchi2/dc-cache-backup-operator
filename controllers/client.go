package controllers

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

func NewKubeClient() *kubernetes.Clientset {
	// Load Kubernetes configuration from the default location
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil
	}

	return clientset
}
