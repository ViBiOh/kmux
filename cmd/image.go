package cmd

import (
	"context"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Get image name of containers for a given deployment",
	Run: func(cmd *cobra.Command, args []string) {
		deploymentName := args[0]

		clients.execute(func(contextName string, client kubeClient) error {
			deployment, err := client.clientset.AppsV1().Deployments(client.namespace).Get(context.Background(), deploymentName, v1.GetOptions{})
			if err != nil {
				return err
			}

			for _, container := range deployment.Spec.Template.Spec.Containers {
				outputStd(contextName, "%s", container.Image)
			}

			return nil
		})
	},
}
