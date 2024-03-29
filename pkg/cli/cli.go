package cli

import (
	"github.com/ThunderAl197/kubedump/pkg/manifests"
	"github.com/ThunderAl197/kubedump/pkg/volumes"
	"github.com/urfave/cli"
	"log"
	"os"
)

func RunCli() {
	app := &cli.App{
		Name:        "kubedump",
		Description: "Kubernetes cluster backup tool",
		Commands: []cli.Command{
			manifests.GetCliCommand(),
			volumes.GetCliCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
