package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// kinds accepted by each command, in completion order.
var (
	podTemplateKinds = []string{"cronjobs", "daemonsets", "deployments", "jobs", "pods", "replicasets", "statefulsets"}
	logKinds         = []string{"cronjobs", "daemonsets", "deployments", "jobs", "namespaces", "nodes", "pods", "services"}
	forwardKinds     = []string{"daemonsets", "deployments", "pods", "services", "statefulsets"}
	restartKinds     = []string{"daemonsets", "deployments", "jobs", "statefulsets"}
	scaleKinds       = []string{"deployments", "replicasets", "statefulsets"}
)

// commandContext cancels as soon as the user interrupts the command.
func commandContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
}

// resourceCompletion completes the kind then the names of that kind, gathered
// from every configured context.
func resourceCompletion(kinds ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return kinds, cobra.ShellCompDirectiveNoFileComp

		case 1:
			lister, err := resource.ListerFor(args[0])
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			clients, err = getKubernetesClient(viper.GetStringSlice("context"))
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			return listObjects(cmd.Context(), viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp

		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// checkSingleNamespace rejects `--all-namespaces` for commands targeting one
// named resource, the namespace is needed to find it.
func checkSingleNamespace(cmd *cobra.Command) error {
	if allNamespace {
		return fmt.Errorf("`%s` needs a namespace, it cannot run with `--all-namespaces`", cmd.Name())
	}

	return nil
}

// compileContainerFilter compiles the shared `--container` flag, if any.
func compileContainerFilter() error {
	containerRegexp = nil

	if len(container) == 0 {
		return nil
	}

	var err error

	containerRegexp, err = regexp.Compile(container)
	if err != nil {
		return fmt.Errorf("container filter compile: %w", err)
	}

	return nil
}
