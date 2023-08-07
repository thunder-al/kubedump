package manifest_loader

import (
	"context"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
)

type Config struct {
	Kubeconfig        string
	OutputDir         string
	FileTemplate      string
	OnlyNamespaces    []string
	ExcludeNamespaces []string
	NoNonNamespaced   bool
	OnlyResources     []string
	ExcludeResources  []string
	DryRun            bool
}

func Dump(cfg *Config) error {
	var err error

	ctx := context.Background()

	err = InitClient(cfg.Kubeconfig)
	if err != nil {
		return err
	}

	var g errgroup.Group

	groupsChannel := make(chan ResourceGroup)

	g.Go(func() error {
		defer close(groupsChannel)
		err := DiscoverGroups(ctx, groupsChannel)
		return err
	})

	resourceChannel := make(chan ResourceAndGroup, 15)

	g.Go(func() error {
		defer close(resourceChannel)

		for group := range groupsChannel {
			if cfg.NoNonNamespaced && group.Namespaced {
				continue
			}

			resourceName := group.Resource

			if len(cfg.ExcludeResources) > 0 {
				exclude := false
				for _, rs := range cfg.ExcludeResources {
					if rs == resourceName {
						log.Printf("Skipping %s resource bacause of exclude option\n", resourceName)
						exclude = true
						break
					}
				}
				if exclude {
					continue
				}
			}

			if len(cfg.OnlyResources) > 0 {
				exclude := true
				for _, rs := range cfg.OnlyResources {
					if rs == resourceName {
						exclude = false
					}
				}
				if exclude {
					log.Printf("Skipping %s resource bacause of inclusive option\n", resourceName)
					continue
				}
			}

			log.Printf("Loading %s resource\n", resourceName)

			err := DiscoverResources(ctx, group, resourceChannel)
			if err != nil {
				return err
			}
		}

		return nil
	})

	g.Go(func() error {
		for res := range resourceChannel {
			resNs := res.resource.GetNamespace()

			if len(cfg.ExcludeNamespaces) > 0 {
				exclude := false
				for _, ns := range cfg.ExcludeNamespaces {
					if ns == resNs {
						exclude = true
						break
					}
				}
				if exclude {
					continue
				}
			}

			if len(cfg.OnlyNamespaces) > 0 {
				exclude := true
				for _, ns := range cfg.OnlyNamespaces {
					if ns == resNs {
						exclude = false
					}
				}
				if exclude {
					continue
				}
			}

			fileName := getResourceFilePath(cfg, res)
			fileData, err := serializeObject(cfg, res)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				log.Printf("[dry run] %s %d bytes\n", fileName, len(fileData))
			} else {
				var err error
				filePath := path.Join(cfg.OutputDir, fileName)

				err = os.MkdirAll(path.Dir(filePath), 0700)
				if err != nil {
					return err
				}

				err = os.WriteFile(filePath, fileData, 0600)
				if err != nil {
					return err
				}
			}

		}

		return nil
	})

	return g.Wait()
}

func getResourceFilePath(cfg *Config, res ResourceAndGroup) string {
	filePath := cfg.FileTemplate

	namespace := res.resource.GetNamespace()

	if !res.group.Namespaced {
		namespace = "_cluster"
	}

	filePath = strings.NewReplacer(
		"{namespace}", removeIllegalFileChars(namespace),
		"{kind}", removeIllegalFileChars(res.resource.GetKind()),
		"{resource}", removeIllegalFileChars(res.group.Resource),
		"{name}", removeIllegalFileChars(res.resource.GetName()),
	).Replace(filePath)

	return filePath
}

func removeIllegalFileChars(fileName string) string {
	return regexp.MustCompile("[^A-Za-z0-9._-]").ReplaceAllString(fileName, "-")
}

func serializeObject(cfg *Config, res ResourceAndGroup) ([]byte, error) {

	obj := res.resource

	obj.SetManagedFields(nil)

	payload, err := yaml.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}

	return payload, nil
}
