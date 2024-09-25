# GoDistribute

GoDistribute is a CLI tool for running distributed tasks across multiple nodes. It provides a simple way to set up and manage nodes, and execute commands in parallel across the configured infrastructure.

## Features

- Server setup automation
- Distributed task execution
- Podman-based containerization
- SSH-based communication
- YAML configuration

## Prerequisites

- Go 1.22.2 or later
- SSH access to target nodes
- `curl` installed on target nodes
- Sudo permissions on target nodes

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/GoDistribute.git
   cd GoDistribute
   ```

2. Build the project:
   ```
   go build .
   ```

## Usage

### Seting up Nodes

Before running any task on the nodes, we need to set up nodes with GoDistribute:

```
./GoDistribute setup --config nodes.yaml
```

This command will:
- Check prerequisites on each server
- Install Podman
- Configure Podman and related components

### Running Distributed Tasks

Assuming you have your container locally, we need to archive it:

```
docker save -o /tmp/ubuntu2204.tar ubuntu:22.04 
```

In the next step, we can run distributed tasks across configured nodes:

```
seq 10 | ./GoDistribute run --command "bash -c 'echo {}'" --config nodes.yaml --jobs-per-node 2 --image-tar /tmp/ubuntu2204.tar
```


Options:
- `--command`: The command to run on remote nodes (required)
- `--config`: Path to the server configuration YAML file (required)
- `--jobs-per-node`: Number of jobs to run per server 
- `--image-tar`: Path to a local Docker image tar file to use for the task
- `--show-percentage`: Display job completion percentage

## Configuration

Create a `nodes.yaml` file to define your server infrastructure:

```
hostname: server1.example.com
    - hostname: server1.example.com
      port: 22
      username: user1
      ssh_key_file: /path/to/private_key1
```


## Development

The project structure is as follows:

- `main.go`: Entry point of the application
- `cmd/`: Contains command implementations
  - `root.go`: Root command setup
  - `setup.go`: Server setup command
  - `run.go`: Distributed task execution command
- `config/`: Configuration handling


## License

MIT
