package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

var clients kubeClients

var rootCmd = &cobra.Command{
	Use:   "kube",
	Short: "Kube simplify use of kubectl",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error
		clients, err = getKubernetesClient(strings.Split(viper.GetString("context"), ","))
		if err != nil {
			outputErrAndExit("%s", err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		clients.execute(func(context string, client kubeClient) error {
			info, err := client.clientset.Discovery().ServerVersion()
			if err != nil {
				return fmt.Errorf("get server version: %w", err)
			}

			outputStd(context, "Cluster version: %s\nNamespace: %s", info, client.namespace)

			return nil
		})
	},
}

func getKubernetesClient(contexts []string) (map[string]kubeClient, error) {
	output := make(map[string]kubeClient)

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

		clientset, err := kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			return nil, fmt.Errorf("create kubernetes client: %w", err)
		}

		output[context] = kubeClient{
			clientset: clientset,
			namespace: namespace,
		}
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
		outputErrAndExit("bind `kubeconfig` flag: %s", err)
	}

	flags.String("context", "", "Kubernetes context, comma separated for mutiplexing commands")
	if err := viper.BindPFlag("context", flags.Lookup("context")); err != nil {
		outputErrAndExit("unable bind `context` flag: %s", err)
	}
	if err := viper.BindEnv("context", "KUBECONTEXT"); err != nil {
		outputErrAndExit("unable bind env `KUBECONTEXT`: %s", err)
	}

	flags.String("namespace", "", "Override kubernetes namespace in context")
	if err := viper.BindPFlag("namespace", flags.Lookup("namespace")); err != nil {
		outputErrAndExit("unable bind `namespace` flag: %s", err)
	}

	rootCmd.AddCommand(imageCmd)
	rootCmd.AddCommand(logCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
