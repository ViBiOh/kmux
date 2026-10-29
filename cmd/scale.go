package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	scaleFactor float64
	scaleForce  bool
)

var scaleCmd = &cobra.Command{
	Use:               "scale TYPE NAME",
	Short:             "Scale a resource by a given factor",
	ValidArgsFunction: resourceCompletion(scaleKinds...),
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSingleNamespace(cmd); err != nil {
			return err
		}

		kind := args[0]
		name := args[1]

		if math.IsNaN(scaleFactor) || math.IsInf(scaleFactor, 0) || scaleFactor < 0 {
			return fmt.Errorf("scale factor must be a finite number, zero or more, got %g", scaleFactor)
		}

		if scaleFactor == 0 && !scaleForce {
			return errors.New("use `--force` to confirm downscaling to zero pods")
		}

		ctx, cancel := commandContext(cmd)
		defer cancel()

		return clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			scale, err := resource.GetScale(ctx, kube, kind, name)
			if err != nil {
				return err
			}

			oldReplicas := scale.Spec.Replicas

			scale.Spec.Replicas, err = scaledReplicas(oldReplicas, scaleFactor)
			if err != nil {
				return err
			}

			if oldReplicas == scale.Spec.Replicas {
				kube.Std("No replica change from %d", scale.Spec.Replicas)

				return nil
			}

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

			default:
				return fmt.Errorf("unhandled resource type `%s` for scale", kind)
			}
		})
	},
}

func scaledReplicas(current int32, factor float64) (int32, error) {
	if factor == 0 {
		return 0, nil
	}

	replicas := math.Ceil(float64(max(1, current)) * factor)
	if replicas > math.MaxInt32 {
		return 0, fmt.Errorf("scaling %d by %g overflows the replica count", current, factor)
	}

	return int32(replicas), nil
}

func initScale() {
	flags := scaleCmd.Flags()

	flags.Float64VarP(&scaleFactor, "factor", "", 1, "Scale factor, e.g. 0 to go down to zero, 1.5 for 50% more, 2 to double the size")
	flags.BoolVarP(&scaleForce, "force", "", false, "Acknowledge downscaling to zero")
}
