package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

var user string

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
	Use:               "restart TYPE NAME",
	Short:             "Restart the given resource",
	ValidArgsFunction: resourceCompletion(restartKinds...),
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSingleNamespace(cmd); err != nil {
			return err
		}

		kind := args[0]
		name := args[1]

		ctx, cancel := commandContext(cmd)
		defer cancel()

		var patch restartPatch
		patch.Spec.Template.Metadata.Annotations = map[string]string{
			"kmux.vibioh.fr/restartedAt": time.Now().Format(time.RFC3339),
		}

		if len(user) != 0 {
			patch.Spec.Template.Metadata.Annotations["kmux.vibioh.fr/restartedBy"] = user
		}

		payload, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("marshal patch: %w", err)
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			switch kind {
			case "ds", "daemonset", "daemonsets":
				_, err := kube.AppsV1().DaemonSets(kube.Namespace).Patch(ctx, name, types.MergePatchType, payload, v1.PatchOptions{})

				return err

			case "deploy", "deployment", "deployments":
				_, err := kube.AppsV1().Deployments(kube.Namespace).Patch(ctx, name, types.MergePatchType, payload, v1.PatchOptions{})

				return err

			case "sts", "statefulset", "statefulsets":
				_, err := kube.AppsV1().StatefulSets(kube.Namespace).Patch(ctx, name, types.MergePatchType, payload, v1.PatchOptions{})

				return err

			case "job", "jobs":
				return replaceJob(ctx, kube, name)

			default:
				return fmt.Errorf("unhandled resource type `%s` for restart", kind)
			}
		})

		return nil
	},
}

func replaceJob(ctx context.Context, kube client.Kube, name string) error {
	job, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	job.Spec.Selector = nil
	delete(job.Spec.Template.Labels, "controller-uid")
	delete(job.Spec.Template.Labels, "batch.kubernetes.io/controller-uid")
	delete(job.Spec.Template.Labels, "job-name")
	delete(job.Spec.Template.Labels, "batch.kubernetes.io/job-name")

	job.ObjectMeta = v1.ObjectMeta{
		Name:        job.Name,
		Namespace:   job.Namespace,
		Labels:      job.Labels,
		Annotations: job.Annotations,
	}
	job.Status = batchv1.JobStatus{}

	propagation := v1.DeletePropagationBackground
	if err = kube.BatchV1().Jobs(kube.Namespace).Delete(ctx, name, v1.DeleteOptions{PropagationPolicy: &propagation}); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}

	if err = waitForJobDeletion(ctx, kube, name); err != nil {
		return fmt.Errorf("wait for deletion: %w", err)
	}

	if _, err = kube.BatchV1().Jobs(kube.Namespace).Create(ctx, job, v1.CreateOptions{}); err != nil {
		return fmt.Errorf("recreate job `%s`, it has been deleted: %w", name, err)
	}

	return nil
}

func waitForJobDeletion(ctx context.Context, kube client.Kube, name string) error {
	return wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, time.Minute, true, func(ctx context.Context) (bool, error) {
		if _, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, name, v1.GetOptions{}); apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, nil
	})
}

func initRestart() {
	flags := restartCmd.Flags()

	flags.StringVarP(&user, "user", "u", os.Getenv("KMUX_USER"), "User added in the restartedBy annotation (read from $KMUX_USER)")
}
