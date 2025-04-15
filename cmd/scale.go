package cmd

import (
	"context"
	"errors"
	"math"
	"strings"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	scaleFactor float64
	scaleForce  bool
)

var scaleCmd = &cobra.Command{
	Use:   "scale TYPE NAME",
	Short: "Scale a resource by a given factor",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"deployments",
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

		if scaleFactor == 1 {
			return nil
		}

		if scaleFactor == 0 && !scaleForce {
			return errors.New("Use `--force` to confirm downscaling to zero pods")
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			scale, err := resource.GetScale(ctx, kube, kind, name)
			if err != nil {
				return err
			}

			oldReplicas := scale.Spec.Replicas
			scale.Spec.Replicas = int32(math.Ceil(float64(oldReplicas) * scaleFactor))

			kube.Std("Scale from %d to %d", oldReplicas, scale.Spec.Replicas)

			switch kind {
			case "deploy", "deployment", "deployments":
				_, err := kube.AppsV1().Deployments(kube.Namespace).UpdateScale(ctx, name, scale, v1.UpdateOptions{})
				return err
			case "rs", "replicaset", "replicasets":
				_, err := kube.AppsV1().ReplicaSets(kube.Namespace).UpdateScale(ctx, name, scale, v1.UpdateOptions{})
				return err
			case "sts", "statefulset", "statefulsets":
				_, err := kube.AppsV1().StatefulSets(kube.Namespace).UpdateScale(ctx, name, scale, v1.UpdateOptions{})
				return err
			}

			return nil
		})

		return nil
	},
}

func initScale() {
	flags := scaleCmd.Flags()

	flags.Float64VarP(&scaleFactor, "factor", "", 1, "Scale factor, e.g. 0 to go down to zero, 1.5 for 50% more, 2 to double the size")
	flags.BoolVarP(&scaleForce, "force", "", false, "Acknowledge downscaling to zero")
}
