package manifests

import (
	"context"
	"github.com/ThunderAl197/kubedump/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"log"
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
		list, err = k8s.KDynClient.
			Resource(schema.GroupVersionResource{Group: res.Group, Version: res.Version, Resource: res.Resource}).
			Namespace("").
			List(ctx, metav1.ListOptions{})
	} else {
		list, err = k8s.KDynClient.
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
	discovery := k8s.KClient.Discovery()
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
