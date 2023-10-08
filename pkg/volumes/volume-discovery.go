package volumes

import (
	"context"
	"errors"
	"fmt"
	"github.com/ThunderAl197/kubedump/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeDiscovery struct {
	pv  *v1.PersistentVolume
	pvc *v1.PersistentVolumeClaim
	pa  *storagev1.VolumeAttachment
	pod []v1.Pod
	dp  []appsv1.Deployment
	sts []appsv1.StatefulSet
	ds  []appsv1.DaemonSet
}

func DiscoverVolume(ctx context.Context, vol v1.PersistentVolume, cmd *CommandArgs) (*VolumeDiscovery, error) {
	var (
		err          error
		attachment   *storagev1.VolumeAttachment
		pods         []v1.Pod
		deployments  []appsv1.Deployment
		statefulsets []appsv1.StatefulSet
		daemonsets   []appsv1.DaemonSet
	)

	if vol.Spec.ClaimRef == nil || vol.Spec.ClaimRef.Kind != "PersistentVolumeClaim" {
		return nil, nil // skipped
	}

	if !k8s.IsIncluded(vol.Spec.ClaimRef.Namespace, cmd.OnlyNamespaces, cmd.ExcludeNamespaces) {
		return nil, nil // skipped
	}

	attachments, err := k8s.KClient.StorageV1().
		VolumeAttachments().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, a := range attachments.Items {
		if a.Spec.Source.PersistentVolumeName != nil && *a.Spec.Source.PersistentVolumeName == vol.Name {
			attachment = &a
			break
		}
	}

	if attachment == nil && cmd.IgnoreUnbound {
		return nil, nil // skipped
	}

	pvc, err := k8s.KClient.CoreV1().
		PersistentVolumeClaims(vol.Spec.ClaimRef.Namespace).
		Get(ctx, vol.Spec.ClaimRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	podList, err := k8s.KClient.CoreV1().
		Pods(vol.Spec.ClaimRef.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range podList.Items {
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvc.Name {
				pods = append(pods, pod)
			}
		}
	}

	for _, pod := range pods {
		for _, ref := range pod.OwnerReferences {
			switch ref.Kind {
			case "ReplicaSet":
				rs, err := k8s.KClient.AppsV1().
					ReplicaSets(pod.Namespace).
					Get(ctx, ref.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				for _, ref := range rs.OwnerReferences {
					switch ref.Kind {
					case "Deployment":
						dp, err := k8s.KClient.AppsV1().
							Deployments(pod.Namespace).
							Get(ctx, ref.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						deployments = append(deployments, *dp)
					}
				}

			case "StatefulSet":
				sts, err := k8s.KClient.AppsV1().
					StatefulSets(pod.Namespace).
					Get(ctx, ref.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				statefulsets = append(statefulsets, *sts)

			case "DaemonSet":
				ds, err := k8s.KClient.AppsV1().
					DaemonSets(pod.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				daemonsets = append(daemonsets, *ds)
			}
		}
	}

	return &VolumeDiscovery{
		pv:  &vol,
		pvc: pvc,
		pa:  attachment,
		pod: pods,
		dp:  deployments,
		sts: statefulsets,
		ds:  daemonsets,
	}, nil
}

func DiscoverVolumes(ctx context.Context, cmd *CommandArgs) (map[string]*VolumeDiscovery, error) {
	var (
		err       error
		discovery = make(map[string]*VolumeDiscovery)
	)

	if len(cmd.Resources) > 0 {

		for _, res := range cmd.Resources {
			var vol *v1.PersistentVolume

			vol, err = k8s.KClient.CoreV1().
				PersistentVolumes().
				Get(ctx, res, metav1.GetOptions{})

			// can be pvc name
			if err != nil {
				pvc, err := k8s.KClient.CoreV1().
					PersistentVolumeClaims("").
					Get(ctx, res, metav1.GetOptions{})

				if err != nil {
					return nil, errors.New(fmt.Sprintf("Volume or pvc \"%s\" not found", res))
				}

				if pvc.Spec.VolumeName == "" {
					return nil, errors.New(fmt.Sprintf("PVC \"%s\" not bound", res))
				}

				vol, err = k8s.KClient.CoreV1().
					PersistentVolumes().
					Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
			}

			d, err := DiscoverVolume(ctx, *vol, cmd)
			if err != nil {
				return nil, err
			}

			discovery[res] = d
		}

		return discovery, nil

	} else {

		pv, err := k8s.KClient.CoreV1().
			PersistentVolumes().
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, vol := range pv.Items {
			d, err := DiscoverVolume(ctx, vol, cmd)
			if err != nil {
				return nil, err
			}

			discovery[vol.Name] = d
		}
	}

	return discovery, nil
}
