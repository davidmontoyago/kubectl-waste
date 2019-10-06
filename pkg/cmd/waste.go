/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	listWastefulPodsExample = `
	# find all wasteful pods in the current namespace
	%[1]s waste

	# find all wasteful pods in all namespaces
	%[1]s waste --all-namespaces
`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

var (
	clientset        *kubernetes.Clientset
	metricsClientset *metricsclientset.Clientset
)

type CommandOptions struct {
	configFlags *genericclioptions.ConfigFlags

	resultingContext     *api.Context
	resultingContextName string

	userSpecifiedCluster   string
	userSpecifiedContext   string
	userSpecifiedAuthInfo  string
	userSpecifiedNamespace string

	rawConfig      api.Config
	listNamespaces bool
	args           []string

	clientset        *kubernetes.Clientset
	metricsClientset *metricsclientset.Clientset

	namespace string

	genericclioptions.IOStreams
}

// NewCommandOptions provides an instance of CommandOptions with default values
func NewCommandOptions(streams genericclioptions.IOStreams) *CommandOptions {
	return &CommandOptions{
		configFlags: genericclioptions.NewConfigFlags(true),

		IOStreams: streams,
	}
}

// NewCmd provides a cobra command wrapping CommandOptions
func NewCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCommandOptions(streams)

	cmd := &cobra.Command{
		Use:          "waste [flags]",
		Short:        "Find wasteful pods",
		Example:      fmt.Sprintf(listWastefulPodsExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&o.listNamespaces, "list", o.listNamespaces, "if true, print the list of all namespaces in the current KUBECONFIG")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

// Complete sets all information required for updating the current context
func (o *CommandOptions) Complete(cmd *cobra.Command, args []string) error {
	o.args = args

	var err error

	o.namespace = "kube-system"
	o.clientset, err = InitClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return err
	}
	o.metricsClientset, err = InitMetricsClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return err
	}

	o.rawConfig, err = o.configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	o.userSpecifiedNamespace, err = cmd.Flags().GetString("namespace")
	if err != nil {
		return err
	}
	if len(args) > 0 {
		if len(o.userSpecifiedNamespace) > 0 {
			return fmt.Errorf("cannot specify both a --namespace value and a new namespace argument")
		}

		o.userSpecifiedNamespace = args[0]
	}

	// if no namespace argument or flag value was specified, then there
	// is no need to generate a resulting context
	if len(o.userSpecifiedNamespace) == 0 {
		return nil
	}

	o.userSpecifiedContext, err = cmd.Flags().GetString("context")
	if err != nil {
		return err
	}

	o.userSpecifiedCluster, err = cmd.Flags().GetString("cluster")
	if err != nil {
		return err
	}

	o.userSpecifiedAuthInfo, err = cmd.Flags().GetString("user")
	if err != nil {
		return err
	}

	currentContext, exists := o.rawConfig.Contexts[o.rawConfig.CurrentContext]
	if !exists {
		return errNoContext
	}

	o.resultingContext = api.NewContext()
	o.resultingContext.Cluster = currentContext.Cluster
	o.resultingContext.AuthInfo = currentContext.AuthInfo

	// if a target context is explicitly provided by the user,
	// use that as our reference for the final, resulting context
	if len(o.userSpecifiedContext) > 0 {
		o.resultingContextName = o.userSpecifiedContext
		if userCtx, exists := o.rawConfig.Contexts[o.userSpecifiedContext]; exists {
			o.resultingContext = userCtx.DeepCopy()
		}
	}

	// override context info with user provided values
	o.resultingContext.Namespace = o.userSpecifiedNamespace

	if len(o.userSpecifiedCluster) > 0 {
		o.resultingContext.Cluster = o.userSpecifiedCluster
	}
	if len(o.userSpecifiedAuthInfo) > 0 {
		o.resultingContext.AuthInfo = o.userSpecifiedAuthInfo
	}

	// generate a unique context name based on its new values if
	// user did not explicitly request a context by name
	if len(o.userSpecifiedContext) == 0 {
		o.resultingContextName = generateContextName(o.resultingContext)
	}

	return nil
}

func generateContextName(fromContext *api.Context) string {
	name := fromContext.Namespace
	if len(fromContext.Cluster) > 0 {
		name = fmt.Sprintf("%s/%s", name, fromContext.Cluster)
	}
	if len(fromContext.AuthInfo) > 0 {
		cleanAuthInfo := strings.Split(fromContext.AuthInfo, "/")[0]
		name = fmt.Sprintf("%s/%s", name, cleanAuthInfo)
	}

	return name
}

// Validate ensures that all required arguments and flag values are provided
func (o *CommandOptions) Validate() error {
	if len(o.rawConfig.CurrentContext) == 0 {
		return errNoContext
	}
	if len(o.args) > 1 {
		return fmt.Errorf("either one or no arguments are allowed")
	}

	return nil
}

// Run lists all available namespaces on a user's KUBECONFIG or updates the
// current context based on a provided namespace.
func (o *CommandOptions) Run() error {
	corev1Client := o.clientset.CoreV1()
	metricsv1Client := o.metricsClientset.MetricsV1beta1()

	foundPods, err := findPods(o.namespace, corev1Client, metricsv1Client)
	if err != nil {
		return err
	}
	printPods(foundPods)
	return nil
}

func isContextEqual(ctxA, ctxB *api.Context) bool {
	if ctxA == nil || ctxB == nil {
		return false
	}
	if ctxA.Cluster != ctxB.Cluster {
		return false
	}
	if ctxA.Namespace != ctxB.Namespace {
		return false
	}
	if ctxA.AuthInfo != ctxB.AuthInfo {
		return false
	}

	return true
}

// setNamespace receives a "desired" context state and determines if a similar context
// is already present in a user's KUBECONFIG. If one is not, then a new context is added
// to the user's config under the provided destination name.
// The current context field is updated to point to the new context.
func (o *CommandOptions) setNamespace(fromContext *api.Context, withContextName string) error {
	if len(fromContext.Namespace) == 0 {
		return fmt.Errorf("a non-empty namespace must be provided")
	}

	configAccess := clientcmd.NewDefaultPathOptions()

	// determine if we have already saved this context to the user's KUBECONFIG before
	// if so, simply switch the current context to the existing one.
	if existingResultingCtx, exists := o.rawConfig.Contexts[withContextName]; !exists || !isContextEqual(fromContext, existingResultingCtx) {
		o.rawConfig.Contexts[withContextName] = fromContext
	}
	o.rawConfig.CurrentContext = withContextName

	fmt.Fprintf(o.Out, "namespace changed to %q\n", fromContext.Namespace)
	return clientcmd.ModifyConfig(configAccess, o.rawConfig, true)
}
