package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "godistribute",
	Short: "GoDistribute: A CLI for Distributed Task Execution",
	Long:  `GoDistribute is a CLI tool for running distributed tasks across multiple nodes. It simplifies server setup and task execution using SSH and Podman-based containerization.`,
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
