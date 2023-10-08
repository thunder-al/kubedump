package k8s

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

var (
	KConfig    *rest.Config
	KClient    *kubernetes.Clientset
	KDynClient *dynamic.DynamicClient
)

func InitClient(kubeconfig string) error {
	var err error

	KConfig, err = buildConfig(kubeconfig)
	if err != nil {
		return err
	}

	KClient, err = buildClient(KConfig)
	if err != nil {
		return err
	}

	KDynClient, err = dynamic.NewForConfig(KConfig)
	if err != nil {
		return err
	}

	return nil
}

func buildClient(config *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(config)
	return clientset, err
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		if _, err := os.Stat(kubeconfig); err == nil {
			cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		}
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
