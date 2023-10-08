package volumes

import (
	"context"
	"fmt"
	"github.com/Jeffail/tunny"
	"github.com/ThunderAl197/kubedump/pkg/k8s"
	"golang.org/x/sync/errgroup"
	"log"
	"strings"
)

func Download(cfg *CommandArgs) error {
	var err error

	ctx := context.Background()

	err = k8s.InitClient(cfg.Kubeconfig)
	if err != nil {
		return err
	}

	discovery, err := DiscoverVolumes(ctx, cfg)

	for name, vol := range discovery {
		if vol == nil {
			log.Printf("Volume %s skipped\n", name)
			continue
		}

		var resources []string
		for _, pod := range vol.pod {
			resources = append(resources, fmt.Sprintf("pod/%s", pod.Name))
		}
		for _, dp := range vol.dp {
			resources = append(resources, fmt.Sprintf("deployment/%s", dp.Name))
		}
		for _, sts := range vol.sts {
			resources = append(resources, fmt.Sprintf("statefulset/%s", sts.Name))
		}
		for _, ds := range vol.ds {
			resources = append(resources, fmt.Sprintf("daemonset/%s", ds.Name))
		}

		if len(resources) == 0 {
			resources = append(resources, "no resources")
		}

		log.Printf("Volume %s pvc/%s mounted to %s\n", vol.pv.Name, vol.pvc.Name, strings.Join(resources, ", "))
	}

	if cfg.DryRun {
		return nil
	}

	count := 0
	for _, vol := range discovery {
		if vol == nil {
			continue
		}
		count++
	}

	log.Printf("Downloading %d volumes with %d threads\n\n", count, cfg.Threads)

	pool := tunny.NewFunc(cfg.Threads, func(payload interface{}) interface{} {
		vol := payload.(*VolumeDiscovery)

		log.Printf("Downloading volume %s\n", vol.pv.Name)
		downloader := NewDownloader(vol)
		err = downloader.Download(ctx, cfg)
		if err != nil {
			return err
		}

		return nil
	})
	defer pool.Close()

	var g errgroup.Group

	for _, vol := range discovery {
		if vol == nil {
			continue
		}

		v := &*vol
		g.Go(func() error {
			payload := pool.Process(v)
			if payload != nil {
				log.Printf("Fail to download %s: %s", v.pv.Name, payload.(error))
			}

			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return err
	}

	return nil

}
