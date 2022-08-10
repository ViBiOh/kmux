package cmd

import (
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/ViBiOh/kube/pkg/client"
	"github.com/ViBiOh/kube/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var imageCmd = &cobra.Command{
	Use:   "image <resource_type> <resource_name>",
	Short: "Get image name of containers for a given resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"cronjobs",
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

			return getCommonObjects(viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.ExactValidArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		resourceName := args[1]

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		clients.Execute(func(kube client.Kube) error {
			var containers []v1.Container

			switch resourceType {
			case "cronjob", "cronjobs":
				item, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				containers = item.Spec.JobTemplate.Spec.Template.Spec.Containers
			case "ds", "daemonset", "daemonsets":
				item, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				containers = item.Spec.Template.Spec.Containers
			case "deploy", "deployment", "deployments":
				item, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				containers = item.Spec.Template.Spec.Containers
			case "job", "jobs":
				item, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				containers = item.Spec.Template.Spec.Containers
			case "sts", "statefulset", "statefulsets":
				item, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				containers = item.Spec.Template.Spec.Containers
			default:
				return fmt.Errorf("unhandled resource type `%s` for image", resourceType)
			}

			for _, container := range containers {
				kube.Std("%s", container.Image)
			}

			return nil
		})
	},
}
