/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package config

import (
	"fmt"
	"io"

	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
)

type CurrentContextOptions struct {
	ConfigAccess ConfigAccess
}

const (
	current_context_long    = `Displays the current-context`
	current_context_example = `# Display the current-context
$ kubectl config current-context`
)

func NewCmdConfigCurrentContext(out io.Writer, configAccess ConfigAccess) *cobra.Command {
	options := &CurrentContextOptions{ConfigAccess: configAccess}

	cmd := &cobra.Command{
		Use:     "current-context",
		Short:   "Displays the current-context",
		Long:    current_context_long,
		Example: current_context_example,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCurrentContext(out, args, options)
			cmdutil.CheckErr(err)
		},
	}

	return cmd
}

func RunCurrentContext(out io.Writer, args []string, options *CurrentContextOptions) error {
	config, err := options.ConfigAccess.GetStartingConfig()
	if err != nil {
		return err
	}
	return describeCurrentContext(out, config)
}

func describeCurrentContext(out io.Writer, config *clientcmdapi.Config) error {
	if config.CurrentContext == "" {
		err := fmt.Errorf("current-context is not set\n")
		return err
	}
	fmt.Fprintf(out, "Name:\t%s\n", config.CurrentContext)
	currentContext := config.Contexts[config.CurrentContext]
	if currentContext == nil {
		err := fmt.Errorf("has no current-context named %s\n", config.CurrentContext)
		return err
	}
	fmt.Fprintf(out, "  Cluster:\t%s\n", currentContext.Cluster)
	fmt.Fprintf(out, "  User:\t%s\n", currentContext.AuthInfo)
	fmt.Fprintf(out, "  Namespace:\t%s\n", currentContext.Namespace)
	return nil
}
