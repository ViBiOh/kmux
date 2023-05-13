package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/env"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var envCmd = &cobra.Command{
	Use:   "env TYPE NAME",
	Short: "Get all configured environment variables of containers for a given resource",
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
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceType := args[0]
		resourceName := args[1]

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

		envGetter := env.NewEnvGetter(resourceType, resourceName).
			WithContainerRegexp(containerRegexp)

		clients.Execute(ctx, envGetter.Get)

		return nil
	},
}

func initEnv() {
	flags := envCmd.Flags()

	flags.StringVarP(&container, "container", "c", "", "Filter container's name by regexp, default to all containers")
}
