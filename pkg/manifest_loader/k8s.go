package manifest_loader

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
)

var (
	KConfig    *rest.Config
	KClient    *kubernetes.Clientset
	KDynClient *dynamic.DynamicClient
)

type ResourceGroup struct {
	Group      string
	Version    string
	Resource   string
	Namespaced bool
}

type ResourceAndGroup struct {
	group    ResourceGroup
	resource unstructured.Unstructured
}

func DiscoverResources(ctx context.Context, res ResourceGroup, ch chan<- ResourceAndGroup) error {
	var (
		list *unstructured.UnstructuredList
		err  error
	)

	if res.Namespaced {
		list, err = KDynClient.
			Resource(schema.GroupVersionResource{Group: res.Group, Version: res.Version, Resource: res.Resource}).
			Namespace("").
			List(ctx, metav1.ListOptions{})
	} else {
		list, err = KDynClient.
			Resource(schema.GroupVersionResource{Group: res.Group, Version: res.Version, Resource: res.Resource}).
			List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return err
	}

	for _, obj := range list.Items {
		ch <- ResourceAndGroup{res, obj}
	}

	return nil
}

func DiscoverGroups(ctx context.Context, ch chan<- ResourceGroup) error {
	discovery := KClient.Discovery()
	groupList, err := discovery.ServerGroups()
	if err != nil {
		return err
	}

	for _, group := range groupList.Groups {
		resourceList, err := discovery.ServerResourcesForGroupVersion(group.PreferredVersion.GroupVersion)
		if err != nil {
			log.Printf("Cannot discover group %s\n", group.PreferredVersion.GroupVersion)
			continue
		}

		for _, resource := range resourceList.APIResources {

			canList := false

			for _, verb := range resource.Verbs {
				if verb == "list" {
					canList = true
				}
			}

			if !canList {
				continue
			}

			ch <- ResourceGroup{
				Group:      group.Name,
				Version:    group.PreferredVersion.Version,
				Resource:   resource.Name,
				Namespaced: resource.Namespaced,
			}
		}
	}

	return nil
}

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
