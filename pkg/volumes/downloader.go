package volumes

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ThunderAl197/kubedump/pkg/k8s"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"os"
	"sigs.k8s.io/yaml"
	"time"
)

type Downloader struct {
	discovery *VolumeDiscovery
}

func NewDownloader(discovery *VolumeDiscovery) *Downloader {
	return &Downloader{discovery: discovery}
}

func (d *Downloader) Download(ctx context.Context, cfg *CommandArgs) error {
	var err error

	pod, err := d.spawnPod(ctx, cfg)
	if err != nil {
		return err
	}

	log.Printf("Pod %s spawned\n", pod.Name)
	defer func() {
		err = d.deletePod(ctx, pod)
		if err != nil {
			log.Printf("Error deleting pod %s: %s\n", pod.Name, err)
		}
	}()

	log.Printf("Waiting for pod %s to be ready\n", pod.Name)
	err = d.waitPodReady(ctx, pod)
	if err != nil {
		return err
	}

	log.Printf("Downloading volume %s with tar exec\n", d.discovery.pv.Name)
	err = d.downloadWithTar(ctx, pod, cfg)
	if err != nil {
		return err
	}

	log.Printf("Volume %s downloaded\n", d.discovery.pv.Name)

	return nil
}

func (d *Downloader) spawnPod(ctx context.Context, cfg *CommandArgs) (*v1.Pod, error) {
	podName := fmt.Sprintf("kubedump-%s", d.discovery.pv.Name)

	podAffinity := &v1.Affinity{}

	if d.discovery.pv.Spec.NodeAffinity != nil {
		podAffinity.NodeAffinity = &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: d.discovery.pv.Spec.NodeAffinity.Required,
		}
	} else {
		attachments, err := k8s.KClient.StorageV1().
			VolumeAttachments().
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		var va *storagev1.VolumeAttachment

		for _, a := range attachments.Items {
			if a.Spec.Source.PersistentVolumeName != nil && *a.Spec.Source.PersistentVolumeName == d.discovery.pv.Name {
				va = &a
				break
			}
		}

		if va == nil {
			return nil, fmt.Errorf("volume attachment for pv %s not found", d.discovery.pv.Name)
		}

		podAffinity.NodeAffinity = &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchFields: []v1.NodeSelectorRequirement{
							{
								Key:      "metadata.name",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{va.Spec.NodeName},
							},
						},
					},
				},
			},
		}
	}

	gracePeriod := int64(0)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: d.discovery.pvc.Namespace,
		},
		Spec: v1.PodSpec{
			Affinity:                      podAffinity,
			RestartPolicy:                 v1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &gracePeriod,
			Containers: []v1.Container{
				{
					Name:    "kubedump",
					Image:   "debian:bookworm",
					Command: []string{"tail", "-f", "/dev/null"},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "vol",
							MountPath: "/mnt/vol",
							ReadOnly:  true,
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							"cpu":    resource.MustParse("500m"),
							"memory": resource.MustParse("1000Mi"),
						},
						Requests: v1.ResourceList{
							"cpu":    resource.MustParse("0m"),
							"memory": resource.MustParse("0Mi"),
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "vol",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: d.discovery.pvc.Name,
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	createdPod, err := k8s.KClient.CoreV1().
		Pods(d.discovery.pvc.Namespace).
		Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdPod, nil
}

func (d *Downloader) isPodReady(ctx context.Context, pod *v1.Pod) (bool, error) {
	pod, err := k8s.KClient.CoreV1().
		Pods(pod.Namespace).
		Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if pod.Status.Phase != v1.PodRunning {
		return false, nil
	}

	return true, nil
}

func (d *Downloader) waitPodReady(ctx context.Context, pod *v1.Pod) error {
	threshold := time.Now().Add(time.Minute * 5)

	for {
		ready, err := d.isPodReady(ctx, pod)
		if err != nil {
			return err
		}

		if ready {
			return nil
		}

		if time.Now().After(threshold) {
			return fmt.Errorf("pod %s not ready after 5 min", pod.Name)
		}
	}
}

func (d *Downloader) deletePod(ctx context.Context, pod *v1.Pod) error {
	err := k8s.KClient.CoreV1().
		Pods(pod.Namespace).
		Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (d *Downloader) downloadWithTar(ctx context.Context, pod *v1.Pod, cfg *CommandArgs) error {

	var (
		err      error
		command  = []string{"bash", "-c", "((tar -cf - -C /mnt/vol .; dd if=/dev/zero of=/dev/stdout bs=1024 count=1024 status=none) | gzip -4cf) && sleep 5 || echo error >&2"}
		destDir  = fmt.Sprintf("%s/volumes", cfg.OutputDir)
		destFile = fmt.Sprintf("%s/%s.tar.gz", destDir, d.discovery.pv.Name)
	)

	// create output dir
	err = os.MkdirAll(destDir, 0700)
	if err != nil {
		return err
	}

	// save pv manifest
	{
		pv := d.discovery.pv.DeepCopy()
		pv.ManagedFields = nil
		manifestData, err := yaml.Marshal(pv)
		if err != nil {
			return err
		}

		fileName := fmt.Sprintf("%s/%s.yaml", destDir, d.discovery.pv.Name)
		err = os.WriteFile(fileName, manifestData, 0600)
		if err != nil {
			return err
		}
	}

	// save pvc manifest
	if d.discovery.pvc != nil {
		pvc := d.discovery.pvc.DeepCopy()
		pvc.ManagedFields = nil
		manifestData, err := yaml.Marshal(pvc)
		if err != nil {
			return err
		}

		fileName := fmt.Sprintf("%s/volumes/%s-pvc.yaml", cfg.OutputDir, d.discovery.pv.Name)
		err = os.WriteFile(fileName, manifestData, 0600)
		if err != nil {
			return err
		}
	}

	// create archive file
	archiveFile, err := os.OpenFile(destFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(archiveFile)

	// stream tar+gzip to file
	req := k8s.KClient.CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(
			&v1.PodExecOptions{
				Command:   command,
				Container: "kubedump",
				Stdout:    true,
				Stderr:    true,
				Stdin:     false,
				TTY:       false,
			},
			scheme.ParameterCodec,
		)

	exec, err := remotecommand.NewSPDYExecutor(k8s.KConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: writer,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	err = archiveFile.Close()
	if err != nil {
		log.Printf("Error closing archive file: %s\n", err)
	}

	return nil
}
