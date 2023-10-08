package manifests

import (
	"github.com/urfave/cli"
	"k8s.io/client-go/util/homedir"
	"path"
)

func GetCliCommand() cli.Command {
	return cli.Command{
		Name:  "manifests",
		Usage: "Download cluster manifests",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "kubeconfig",
				EnvVar: "KUBECONFIG",
				Usage:  "Path to kubeconfig file",
				Value:  path.Join(homedir.HomeDir(), ".kube", "config"),
			},
			cli.StringFlag{
				Name:  "output,o",
				Usage: "Path to dist directory",
				Value: "./out",
			},
			cli.StringFlag{
				Name:  "template,t",
				Usage: "File name template. Available patterns: {namespace}, {kind}, {resource}, {name}. Non-namespaced will have _cluster in {namespace}",
				Value: "manifests/{namespace}/{resource}/{name}.yaml",
			},
			cli.StringSliceFlag{
				Name:  "namespaces,n",
				Usage: "Load specific namespaces. By default all",
			},
			cli.StringSliceFlag{
				Name:  "exclude-namespaces,N",
				Usage: "Load other except this namespaces. Can work with --namespaces",
			},
			cli.StringSliceFlag{
				Name:  "resources,r",
				Usage: "Load specific namespaces. By default all (see --exclude-resources)",
			},
			cli.StringSliceFlag{
				Name:  "exclude-resources,R",
				Usage: "Load other except this namespaces. Can work with --resource. By default: events",
				Value: &cli.StringSlice{"events", "componentstatuses"},
			},
			cli.BoolFlag{
				Name:  "no-non-namespaced,G",
				Usage: "Dont load non-namespaced resources",
			},
			cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Dont write files on disk",
			},
		},
		Action: func(c *cli.Context) error {
			var err error

			err = Dump(&Config{
				Kubeconfig:        c.String("kubeconfig"),
				OutputDir:         c.String("output"),
				FileTemplate:      c.String("template"),
				OnlyNamespaces:    c.StringSlice("namespaces"),
				ExcludeNamespaces: c.StringSlice("exclude-namespaces"),
				NoNonNamespaced:   c.Bool("no-non-namespaced"),
				OnlyResources:     c.StringSlice("resources"),
				ExcludeResources:  c.StringSlice("exclude-resources"),
				DryRun:            c.Bool("dry-run"),
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
