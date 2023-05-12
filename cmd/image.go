package cmd

import (
	"context"
	"strings"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var imageCmd = &cobra.Command{
	Use:   "image TYPE NAME",
	Short: "Get all image names of containers for a given resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"cronjobs",
				"daemonsets",
				"deployments",
				"jobs",
				"pods",
				"replicasets",
				"statefulsets",
			}, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			lister, err := resource.ListerFor(args[0])
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			clients, err = getKubernetesClient(strings.Split(viper.GetString("context"), ","))
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			return getCommonObjects(cmd.Context(), viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		resourceName := args[1]

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			podTemplate, err := resource.PodSpecGetter(ctx, kube, resourceType, resourceName)
			if err != nil {
				return err
			}

			for _, container := range podTemplate.InitContainers {
				kube.Std("%s", container.Image)
			}

			for _, container := range podTemplate.Containers {
				kube.Std("%s", container.Image)
			}

			return nil
		})
	},
}
