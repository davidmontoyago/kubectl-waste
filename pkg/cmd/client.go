package cmd

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"log"
	"os"
	"path/filepath"

	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	kubeconfig string
)

func init() {
	kubeconfig = filepath.Join(
		os.Getenv("HOME"), ".kube", "config",
	)
}

func buildConfig() *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func InitClient() (*kubernetes.Clientset, error) {
	config := buildConfig()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func InitMetricsClient() (*metrics.Clientset, error) {
	config := buildConfig()
	metricsClientset, err := metrics.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return metricsClientset, nil
}
