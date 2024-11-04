package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/forward"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/ViBiOh/kmux/pkg/tcpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var limiter uint

var portForwardCmd = &cobra.Command{
	Use:     "port-forward TYPE NAME [local_port:]remote_port",
	Aliases: []string{"forward"},
	Short:   "Port forward to pods of a resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"daemonsets",
				"deployments",
				"pods",
				"services",
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
	Args: cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceType := args[0]
		resourceName := args[1]
		rawPort := args[2]

		ports := strings.SplitN(rawPort, ":", 2)

		localPort, err := strconv.ParseUint(ports[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid local port: %s", ports[0])
		}

		var remotePort string
		if len(ports) == 2 {
			remotePort = ports[1]
		} else {
			remotePort = ports[0]
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		output.Std("", "Listening tcp on %d", localPort)

		var pool *tcpool.Pool
		if !dryRun {
			pool = tcpool.New()
			go pool.Start(ctx, localPort)
		}

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		forwarder := forward.NewForwarder(resourceType, resourceName, remotePort, pool, limiter)

		clients.Execute(ctx, forwarder.Forward)

		if pool != nil {
			<-pool.Done()
		}

		return nil
	},
}

func initPortForward() {
	flags := portForwardCmd.Flags()

	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
	flags.UintVarP(&limiter, "limit", "l", 0, "Limit forward to only n pods")
}
