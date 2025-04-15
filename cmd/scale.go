package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var scaleFactor float64

type scalePatch struct {
	Spec struct {
		Replicas int32 `json:"replicas"`
	} `json:"spec"`
}

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

		if scaleFactor == 0 {
			return errors.New("no scale factor provided")
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			replicable, err := resource.GetReplicable(ctx, kube, kind, name)
			if err != nil {
				return err
			}

			replicas := *replicable.GetReplicas()

			var patch scalePatch
			patch.Spec.Replicas = replicas + int32(math.Ceil(float64(replicas)*scaleFactor))

			payload, err := json.Marshal(patch)
			if err != nil {
				return fmt.Errorf("marshal patch: %w", err)
			}

			kube.Std("Scale from %d to %d", replicas, patch.Spec.Replicas)

			switch kind {
			case "deploy", "deployment", "deployments":
				_, err := kube.AppsV1().Deployments(kube.Namespace).Patch(ctx, name, types.MergePatchType, payload, v1.PatchOptions{})
				return err
			case "rs", "replicaset", "replicasets":
				_, err := kube.AppsV1().ReplicaSets(kube.Namespace).Patch(ctx, name, types.MergePatchType, payload, v1.PatchOptions{})
				return err
			case "sts", "statefulset", "statefulsets":
				_, err := kube.AppsV1().StatefulSets(kube.Namespace).Patch(ctx, name, types.MergePatchType, payload, v1.PatchOptions{})
				return err
			}

			return nil
		})

		return nil
	},
}

func initScale() {
	flags := scaleCmd.Flags()

	flags.Float64VarP(&scaleFactor, "factor", "f", 0, "Scale factor, e.g. -1 to go down to zero, 0.5 for 50%, 1 to double the size")
}
