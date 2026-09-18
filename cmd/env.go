package cmd

import (
	"github.com/ViBiOh/kmux/pkg/env"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:               "env TYPE NAME",
	Short:             "Get all configured environment variables of containers for a given resource",
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

		envGetter := env.NewEnvGetter(kind, name).
			WithContainerRegexp(containerRegexp)

		clients.Execute(ctx, envGetter.Get)

		return nil
	},
}

func initEnv() {
	flags := envCmd.Flags()

	flags.StringVarP(&container, "container", "c", "", "Filter container's name by regexp, default to all containers")
}
