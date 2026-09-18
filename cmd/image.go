package cmd

import (
	"context"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	Use:               "image TYPE NAME",
	Short:             "Get all image names of containers for a given resource",
	ValidArgsFunction: resourceCompletion(podTemplateKinds...),
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSingleNamespace(cmd); err != nil {
			return err
		}

		kind := args[0]
		name := args[1]

		ctx, cancel := commandContext(cmd)
		defer cancel()

		if err := compileContainerFilter(); err != nil {
			return err
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			podSpec, err := resource.GetPodSpec(ctx, kube, kind, name)
			if err != nil {
				return err
			}

			for _, container := range resource.SelectedContainers(podSpec, containerRegexp) {
				kube.Std("%s", container.Image)
			}

			return nil
		})

		return nil
	},
}

func initImage() {
	flags := imageCmd.Flags()

	flags.StringVarP(&container, "container", "c", "", "Filter container's name by regexp, default to all containers")
}
