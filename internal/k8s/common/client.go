package common

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	"os"
	"path/filepath"
)

func loadKubeConfig() (*rest.Config, error) {
	home := homedir.HomeDir()
	kubeconfigPath := filepath.Join(home, ".kube", "config")

	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		kubeconfigPath = kubeconfigEnv
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %w", err)
	}
	return config, nil
}

func GetKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := loadKubeConfig()
	if err != nil {
		return nil, err // Error already formatted by loadKubeConfig
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}
	return clientset, nil
}

// GetMetricsClient creates a Kubernetes metrics clientset.
func GetMetricsClient() (*versioned.Clientset, error) {
	config, err := loadKubeConfig()
	if err != nil {
		// Return nil, but maybe don't error out completely if metrics are optional?
		// Depends on how you want to handle metrics server unavailability.
		// For now, let's return the error.
		return nil, err
	}

	metricsClient, err := versioned.NewForConfig(config)
	if err != nil {
		// It's common for the metrics server to be unavailable.
		// Consider logging a warning instead of returning a fatal error,
		// unless the command *requires* metrics.
		return nil, fmt.Errorf("error creating Metrics client: %w", err)
	}
	return metricsClient, nil
}
