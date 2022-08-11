package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

var (
	clients      client.Array
	allNamespace bool
)

var rootCmd = &cobra.Command{
	Use:   "kmux",
	Short: "Multiplexing kubectl common tasks accross clusters",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error
		clients, err = getKubernetesClient(strings.Split(viper.GetString("context"), ","))
		if err != nil {
			output.Fatal("%s", err)
		}
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		output.Close()
		<-output.Done()
	},
	Run: func(cmd *cobra.Command, args []string) {
		clients.Execute(func(kube client.Kube) error {
			info, err := kube.Discovery().ServerVersion()
			if err != nil {
				return fmt.Errorf("get server version: %w", err)
			}

			kube.Std("Cluster version: %s\nNamespace: %s", info, kube.Namespace)

			return nil
		})
	},
}

func getKubernetesClient(contexts []string) (client.Array, error) {
	var output client.Array

	for _, context := range contexts {
		configOverrides := &clientcmd.ConfigOverrides{
			CurrentContext: context,
			Context: api.Context{
				Namespace: viper.GetString("namespace"),
			},
		}
		configRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: viper.GetString("kubeconfig")}

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configRules, configOverrides)
		k8sConfig, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("read kubernetes config file: %w", err)
		}

		namespace, _, err := clientConfig.Namespace()
		if err != nil {
			return nil, fmt.Errorf("read configured namespace: %w", err)
		}

		if allNamespace {
			namespace = ""
		}

		clientset, err := kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			return nil, fmt.Errorf("create kubernetes client: %w", err)
		}

		output = append(output, client.New(context, namespace, clientset))
	}

	return output, nil
}

func init() {
	viper.AutomaticEnv()

	flags := rootCmd.PersistentFlags()

	var defaultConfig string
	if home := homedir.HomeDir(); home != "" {
		defaultConfig = filepath.Join(home, ".kube", "config")
	}

	flags.String("kubeconfig", defaultConfig, "Kubernetes configuration file")
	if err := viper.BindPFlag("kubeconfig", flags.Lookup("kubeconfig")); err != nil {
		output.Fatal("bind `kubeconfig` flag: %s", err)
	}

	flags.String("context", "", "Kubernetes context, comma separated for mutiplexing commands")
	if err := viper.BindPFlag("context", flags.Lookup("context")); err != nil {
		output.Fatal("bind `context` flag: %s", err)
	}
	if err := viper.BindEnv("context", "KUBECONTEXT"); err != nil {
		output.Fatal("bind env `KUBECONTEXT`: %s", err)
	}

	flags.BoolVarP(&allNamespace, "all-namespaces", "A", false, "Find resources in all namespaces")

	flags.StringP("namespace", "n", "", "Override kubernetes namespace in context")
	if err := viper.BindPFlag("namespace", flags.Lookup("namespace")); err != nil {
		output.Fatal("bind `namespace` flag: %s", err)
	}

	if err := rootCmd.RegisterFlagCompletionFunc("namespace", func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		lister, err := resource.ListerFor("namespace")
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		clients, err = getKubernetesClient(strings.Split(viper.GetString("context"), ","))
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return getCommonObjects("", lister), cobra.ShellCompDirectiveDefault
	}); err != nil {
		output.Fatal("register `namespace` flag completion: %s", err)
	}

	rootCmd.AddCommand(imageCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(watchCmd)

	initLog()
	rootCmd.AddCommand(logCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
