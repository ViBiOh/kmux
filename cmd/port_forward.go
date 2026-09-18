package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ViBiOh/kmux/pkg/forward"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/tcpool"
	"github.com/spf13/cobra"
)

var limiter uint

var portForwardCmd = &cobra.Command{
	Use:               "port-forward TYPE NAME [local_port:]remote_port",
	Aliases:           []string{"forward"},
	Short:             "Port forward to pods of a resource",
	ValidArgsFunction: resourceCompletion(forwardKinds...),
	Args:              cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSingleNamespace(cmd); err != nil {
			return err
		}

		kind := args[0]
		name := args[1]

		localPort, remotePort, err := parsePorts(args[2])
		if err != nil {
			return err
		}

		ctx, cancel := commandContext(cmd)
		defer cancel()

		var pool *tcpool.Pool

		if !dryRun {
			pool = tcpool.New()

			if err := pool.Listen(localPort); err != nil {
				return fmt.Errorf("listen locally: %w", err)
			}

			go pool.Serve(ctx)

			output.Std("", "Listening tcp on %d", localPort)
		}

		forwarder := forward.NewForwarder(kind, name, remotePort, pool, limiter).
			WithDryRun(dryRun)

		clients.Execute(ctx, forwarder.Forward)

		cancel()

		if pool != nil {
			<-pool.Done()
		}

		return nil
	},
}

func parsePorts(rawPort string) (uint64, string, error) {
	local, remote, hasLocal := strings.Cut(rawPort, ":")
	if !hasLocal {
		remote = local
	}

	localPort, err := strconv.ParseUint(local, 10, 16)
	if err != nil {
		return 0, "", fmt.Errorf("invalid local port `%s`", local)
	}

	if len(remote) == 0 {
		return 0, "", fmt.Errorf("invalid remote port `%s`", rawPort)
	}

	return localPort, remote, nil
}

func initPortForward() {
	flags := portForwardCmd.Flags()

	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
	flags.UintVarP(&limiter, "limit", "", 0, "Limit forward to only n pods")
}
