# Swiss Army CLI (swissarmycli)

A multi-purpose CLI tool designed to simplify common tasks for Platform Engineers and DevOps working with Kubernetes and AWS.

## Overview

Swiss Army CLI aims to provide convenient wrappers and combined functionalities that often require multiple `kubectl` and `aws` commands. It streamlines workflows by offering targeted commands for specific operational needs.

## Features

*   **`connect [nodeName]`**: Connect directly to an AWS EC2 instance backing a Kubernetes node using AWS Systems Manager (SSM) Start Session. It automatically looks up the instance ID and region from the node's ProviderID.
*   **`node-usage`**: Display a summary table of resource utilization across all nodes in your Kubernetes cluster. Shows CPU/Memory capacity, total pod requests, total pod limits, and current real-time usage (requires Metrics Server).

*(More features planned!)*

## Prerequisites

Before using `swissarmycli`, ensure you have the following installed and configured:

1.  **Go:** Version 1.18 or higher (for building).
2.  **Kubernetes Cluster Access:** A valid `kubeconfig` file (`~/.kube/config` or specified via `KUBECONFIG` environment variable) pointing to your target cluster.
3.  **AWS CLI:** Required for the `connect` command. Must be installed and in your system's PATH.
4.  **AWS Credentials:** Configure your AWS credentials so the AWS CLI (and potentially future AWS SDK calls) can authenticate. Common methods include:
    *   Environment Variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`)
    *   Shared credential file (`~/.aws/credentials`)
    *   IAM role attached to the EC2 instance or ECS task where you run the CLI.
5.  **Kubernetes Metrics Server (Optional but Recommended):** The `node-usage` command relies on the Metrics Server being deployed in your cluster to show real-time CPU and Memory usage. If not present, usage columns will show "N/A".

## Installation / Building

1.  **Clone the repository (if you haven't already):**
2.  **Build the binary:**
    *   Using Go:
    bash go build -o ./bin/swissarmycli ./cmd/swissarmycli

    *   Using the Makefile (Recommended):
    bash make build
3.  **(Optional) Add to PATH:** Move the `./bin/swissarmycli` binary to a directory included in your system's PATH (e.g., `/usr/local/bin`) for easier access.
bash sudo mv ./bin/swissarmycli /usr/local/bin/

## Usage

### General Help
swissarmycli --help

### Connect to a Node via SSM

This command finds the AWS instance ID associated with the given Kubernetes node name and starts an interactive SSM session.
bash swissarmycli connect <kubernetes-node-name>
Example:
swissarmycli connect ip-10-20-30-40.us-west-2.compute.internal

### Node Usage
swissarmycli node-usage