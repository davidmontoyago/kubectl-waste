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

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	listWastefulPodsExample = `
	# find all wasteful pods across all namespaces
	%[1]s waste

	# find all wasteful pods in a namespace
	%[1]s waste -n my-namespace
`
)

var (
	clientset        *kubernetes.Clientset
	metricsClientset *metricsclientset.Clientset
)

type CommandOptions struct {
	configFlags *genericclioptions.ConfigFlags

	userSpecifiedNamespace string

	args []string

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

	// cmd.Flags().BoolVar(&o.myFlag, "flag-name", o.myFlag, "if true, do something")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

// Complete sets all information required
func (o *CommandOptions) Complete(cmd *cobra.Command, args []string) error {
	o.args = args

	var err error

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

	o.userSpecifiedNamespace, err = cmd.Flags().GetString("namespace")
	if err != nil {
		return err
	}

	return nil
}

// Validate ensures that all required arguments and flag values are provided
func (o *CommandOptions) Validate() error {
	return nil
}

// Run finds and print pods
func (o *CommandOptions) Run() error {
	corev1Client := o.clientset.CoreV1()
	metricsv1Client := o.metricsClientset.MetricsV1beta1()

	foundPods, err := findPods(o.userSpecifiedNamespace, corev1Client, metricsv1Client)
	if err != nil {
		return err
	}
	printPods(foundPods)
	return nil
}
