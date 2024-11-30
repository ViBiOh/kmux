package cmd

import (
	"context"
	"fmt"
	"regexp"
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

			return listObjects(cmd.Context(), viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind := args[0]
		name := args[1]

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		if len(container) != 0 {
			var err error

			containerRegexp, err = regexp.Compile(container)
			if err != nil {
				return fmt.Errorf("container filter compile: %w", err)
			}
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			podSpec, err := resource.GetPodSpec(ctx, kube, kind, name)
			if err != nil {
				return err
			}

			for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
				if !resource.IsContainedSelected(container, containerRegexp) {
					continue
				}

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
