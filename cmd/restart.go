package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type restartPatch struct {
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		} `json:"template"`
	} `json:"spec"`
}

var restartCmd = &cobra.Command{
	Use:   "restart TYPE NAME",
	Short: "Restart the given resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"daemonsets",
				"deployments",
				"jobs",
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

		var patch restartPatch
		patch.Spec.Template.Metadata.Annotations = map[string]string{
			"kmux.vibioh.fr/restartedAt": time.Now().Format(time.RFC3339),
		}

		payload, err := json.Marshal(patch)
		if err != nil {
			output.Err("", "marshal patch: %s", err)
			return
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			switch resourceType {
			case "ds", "daemonset", "daemonsets":
				_, err := kube.AppsV1().DaemonSets(kube.Namespace).Patch(ctx, resourceName, types.MergePatchType, payload, v1.PatchOptions{})
				return err
			case "deploy", "deployment", "deployments":
				_, err := kube.AppsV1().Deployments(kube.Namespace).Patch(ctx, resourceName, types.MergePatchType, payload, v1.PatchOptions{})
				return err
			case "job", "jobs":
				job, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, resourceName, v1.GetOptions{})
				if err != nil {
					return err
				}

				job.Spec.Selector = nil
				job.Spec.Template.ObjectMeta.Labels = nil

				if err = kube.BatchV1().Jobs(kube.Namespace).Delete(ctx, resourceName, v1.DeleteOptions{}); err != nil {
					return err
				}

				_, err = kube.BatchV1().Jobs(kube.Namespace).Create(ctx, job, v1.CreateOptions{})
				return err
			case "sts", "statefulset", "statefulsets":
				_, err := kube.AppsV1().StatefulSets(kube.Namespace).Patch(ctx, resourceName, types.MergePatchType, payload, v1.PatchOptions{})
				return err
			default:
				return fmt.Errorf("unhandled resource type `%s` for restart", resourceType)
			}
		})
	},
}
