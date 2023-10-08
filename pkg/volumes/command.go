package volumes

import (
	"github.com/urfave/cli"
	"k8s.io/client-go/util/homedir"
	"path"
)

type CommandArgs struct {
	Kubeconfig        string
	OutputDir         string
	OnlyNamespaces    []string
	ExcludeNamespaces []string
	Resources         []string
	DryRun            bool
	Threads           int
}

func GetCliCommand() cli.Command {
	return cli.Command{
		Name:  "volumes",
		Usage: "Download cluster volumes",
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
			cli.StringSliceFlag{
				Name:  "namespaces,n",
				Usage: "Download volumes which has pvc/pod in this namespaces. By default all",
			},
			cli.StringSliceFlag{
				Name:  "exclude-namespaces,N",
				Usage: "Exclude volumes which has pvc/pod in this namespaces. Can work with --namespaces",
			},
			cli.IntFlag{
				Name:  "threads,t",
				Usage: "Number of threads to download volumes. By default 3",
				Value: 3,
			},
			cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Only discover volumes and print them",
			},
		},
		ArgsUsage: "pv/pvc names. if its empty - all",
		Action: func(c *cli.Context) error {
			return Download(&CommandArgs{
				Kubeconfig:        c.String("kubeconfig"),
				OutputDir:         c.String("output"),
				OnlyNamespaces:    c.StringSlice("namespaces"),
				ExcludeNamespaces: c.StringSlice("exclude-namespaces"),
				Resources:         c.Args(),
				Threads:           c.Int("threads"),
				DryRun:            c.Bool("dry-run"),
			})
		},
	}
}
