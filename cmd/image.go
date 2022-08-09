package cmd

import (
	"context"

	"github.com/ViBiOh/kube/pkg/client"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var imageCmd = &cobra.Command{
	Use:   "image <deployment_name>",
	Short: "Get image name of containers for a given deployment",
	Args:  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deploymentName := args[0]

		clients.Execute(func(kube client.Kube) error {
			deployment, err := kube.AppsV1().Deployments(kube.Namespace).Get(context.Background(), deploymentName, v1.GetOptions{})
			if err != nil {
				return err
			}

			for _, container := range deployment.Spec.Template.Spec.Containers {
				kube.Std("%s", container.Image)
			}

			return nil
		})
	},
}
