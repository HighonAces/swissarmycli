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

```bash
swissarmycli --help
```

## CLI Command Usage

This section details the usage of available commands within Swiss Army CLI.

### `connect`

Connect to AWS resources.

#### `connect node [nodeName]`

Connects directly to an AWS EC2 instance backing a Kubernetes node using AWS Systems Manager (SSM) Start Session. It automatically looks up the instance ID and region from the node's ProviderID.

*   **Aliases:** `n`, `nd`
*   **Syntax:** `swissarmycli connect node <kubernetes-node-name>`
*   **Example:**
    ```bash
    swissarmycli connect node ip-10-20-30-40.us-west-2.compute.internal
    ```

#### `connect cluster [partial-cluster-name]`

Connects to an EKS cluster. This can be useful for quickly switching contexts or accessing cluster resources. *(Further details on how this connection is established should be added once the functionality is clear, e.g., updates kubeconfig, opens console, etc.)*

*   **Aliases:** `c`, `cl`, `eks`
*   **Syntax:** `swissarmycli connect cluster <partial-cluster-name>`
*   **Example:**
    ```bash
    swissarmycli connect cluster my-eks-cluster-prod
    ```

### `get`

Retrieves information about AWS resources.

#### `get nlb`

Lists Network Load Balancers (NLBs) in a specified AWS region, showing their name, DNS address, and associated IP addresses.

*   **Syntax:** `swissarmycli get nlb --region <aws-region>`
    *   Shorthand: `swissarmycli get nlb -r <aws-region>`
*   **Arguments & Flags:**
    *   `--region`, `-r` (string, **required**): The AWS region to query for NLBs (e.g., `us-east-1`, `eu-west-2`).
*   **Example:**
    ```bash
    swissarmycli get nlb --region us-east-1
    ```
*   **Output Format:**
    ```
    NLB NAME      DNS                                                 IP(s)
    my-nlb-1      my-nlb-1-abcdef1234567890.elb.us-east-1.amazonaws.com   192.0.2.1, 198.51.100.5
    another-nlb   another-nlb-0987654321fedcba.elb.us-east-1.amazonaws.com 203.0.113.10
    ```

### `node-usage`

Displays a summary table of resource utilization across all nodes in your Kubernetes cluster. Shows CPU/Memory capacity, total pod requests, total pod limits, and current real-time usage (requires Metrics Server).

*   **Syntax:** `swissarmycli node-usage`
*   **Example:**
    ```bash
    swissarmycli node-usage
    ```

### `asg-status [ASG_NAME]`

Displays the status of an AWS Auto Scaling Group (ASG), including instance health, lifecycle states, and scaling activities.

*   **Syntax:** `swissarmycli asg-status <asg-name> [flags]`
*   **Arguments:**
    *   `ASG_NAME`: The name of the Auto Scaling Group.
*   **Flags:**
    *   `--region`, `-r`: AWS region where the ASG is located (e.g., `us-east-1`).
    *   `--profile`, `-p`: AWS CLI profile to use for credentials (e.g., `my-aws-profile`).
    *   `--interval`, `-i`: Refresh interval in seconds when streaming (e.g., `10`). Defaults to 5.
    *   `--stream`, `-s`: Continuously stream the ASG status.
*   **Examples:**
    ```bash
    swissarmycli asg-status my-asg-name
    ```
    ```bash
    swissarmycli asg-status my-asg-name --region us-west-2 --profile production
    ```
    ```bash
    swissarmycli asg-status my-asg-name --stream
    ```
    ```bash
    swissarmycli asg-status my-asg-name -s -i 15 -r eu-central-1
    ```

### `validate [filepath]`

Validates the syntax and structure of a given configuration file (e.g., Kubernetes manifests, Helm charts, etc.). *(The specific types of files validated and the nature of validation should be clarified as the feature is developed.)*

*   **Syntax:** `swissarmycli validate <filepath>`
*   **Arguments:**
    *   `filepath`: Path to the file to be validated.
*   **Example:**
    ```bash
    swissarmycli validate ./path/to/your/kubernetes-deployment.yaml
    ```