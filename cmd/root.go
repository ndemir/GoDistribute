package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "godistribute",
	Short: "GoDistribute: A Distributed Task Scheduler",
	Long:  `GoDistribute is a simple CLI tool that can act as a distributed task scheduler which allows you to manage and execute tasks across multiple nodes via SSH.`,
	Example: `
	# Setup Nodes
	GoDistribute setup --config servers.yaml

  	# Basic Usage
	seq 100 | GoDistribute run --config servers.yaml --command "echo -n 'Hello {} from'; hostname"
  
	# Run the command in containers
	seq 100 | GoDistribute run --config servers.yaml --command "echo -n 'Hello {} from'; hostname" --image-tar /tmp/ubuntu2204.tar

  `,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		PrintAppErrorfAndExit("Error: %v", err)
	}
}
