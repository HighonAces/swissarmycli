# Swiss Army CLI (swissarmycli)

A multi-purpose CLI tool designed to simplify common tasks for Platform Engineers and DevOps working with Kubernetes and AWS.

## Overview

Swiss Army CLI aims to provide convenient wrappers and combined functionalities that often require multiple `kubectl` and `aws` commands. It streamlines workflows by offering targeted commands for specific operational needs.

## Features

*   **`connect node [nodeName]`**: Connect directly to an AWS EC2 instance backing a Kubernetes node using AWS Systems Manager (SSM) Start Session.
*   **`connect cluster [partial-cluster-name]`**: Search and connect to EKS clusters across US regions by updating kubeconfig.
*   **`node-usage`**: Display resource utilization summary across all nodes in your Kubernetes cluster.
*   **`asg-status [ASG_NAME]`**: Monitor AWS Auto Scaling Group status with real-time streaming dashboard.
*   **`validate [filepath]`**: Validate YAML configuration files for syntax errors.
*   **`reveal-secret [secret-name]`**: Find, decode, and display Kubernetes secrets across namespaces.
*   **`check-cert [secret-name]`**: Check TLS certificate details and expiry dates from Kubernetes secrets.
*   **`cost-estimate`**: Estimate monthly costs for your current Kubernetes cluster resources.

## Prerequisites

Before using `swissarmycli`, ensure you have the following installed and configured:

1.  **Go:** Version 1.18 or higher (for building).
2.  **Kubernetes Cluster Access:** A valid `kubeconfig` file (`~/.kube/config` or specified via `KUBECONFIG` environment variable) pointing to your target cluster.
3.  **AWS CLI:** Required for the `connect` command. Must be installed and in your system's PATH.
4.  **AWS Credentials:** Configure your AWS credentials so the AWS CLI can authenticate. Common methods include:
    *   Environment Variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`)
    *   Shared credential file (`~/.aws/credentials`)
    *   IAM role attached to the EC2 instance or ECS task where you run the CLI.
5.  **Kubernetes Metrics Server (Optional):** The `node-usage` command relies on the Metrics Server being deployed in your cluster to show real-time CPU and Memory usage. If not present, usage columns will show "N/A".

## Installation / Building

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd swissarmycli
    ```

2.  **Build the binary:**
    *   Using the Makefile (Recommended):
        ```bash
        make build
        ```
    *   Using Go directly:
        ```bash
        go build -o ./bin/swissarmycli ./cmd/swissarmycli
        ```

3.  **(Optional) Add to PATH:**
    ```bash
    sudo mv ./bin/swissarmycli /usr/local/bin/
    ```

## Usage

### General Help

```bash
swissarmycli --help
```

## CLI Command Usage

### `connect`

Connect to AWS resources.

#### `connect node [nodeName]`

Connects directly to an AWS EC2 instance backing a Kubernetes node using AWS Systems Manager (SSM) Start Session. Automatically looks up the instance ID and region from the node's ProviderID.

*   **Aliases:** `n`, `nd`
*   **Syntax:** `swissarmycli connect node <kubernetes-node-name>`
*   **Example:**
    ```bash
    swissarmycli connect node ip-10-20-30-40.us-west-2.compute.internal
    ```

#### `connect cluster [partial-cluster-name]`

Searches for EKS clusters across US regions (us-east-1, us-east-2, us-west-1, us-west-2) matching the partial name and updates kubeconfig for the selected cluster.

*   **Aliases:** `c`, `cl`, `eks`
*   **Syntax:** `swissarmycli connect cluster <partial-cluster-name>`
*   **Example:**
    ```bash
    swissarmycli connect cluster my-eks-cluster-prod
    ```

### `node-usage`

Displays a summary table of resource utilization across all nodes in your Kubernetes cluster. Shows CPU/Memory capacity, total pod requests, total pod limits, and current real-time usage (requires Metrics Server).

*   **Syntax:** `swissarmycli node-usage`
*   **Example:**
    ```bash
    swissarmycli node-usage
    ```

### `asg-status [ASG_NAME]`

Displays the status of an AWS Auto Scaling Group (ASG), including instance health, lifecycle states, and scaling activities. Supports real-time streaming mode with an interactive dashboard.

*   **Syntax:** `swissarmycli asg-status <asg-name> [flags]`
*   **Arguments:**
    *   `ASG_NAME`: The name of the Auto Scaling Group.
*   **Flags:**
    *   `--region`, `-r`: AWS region where the ASG is located (e.g., `us-east-1`).
    *   `--profile`, `-p`: AWS CLI profile to use for credentials (e.g., `my-aws-profile`).
    *   `--interval`, `-i`: Refresh interval in seconds when streaming (default: 5).
    *   `--stream`, `-s`: Launch interactive monitor stream.
*   **Examples:**
    ```bash
    swissarmycli asg-status my-asg-name
    swissarmycli asg-status my-asg-name --region us-west-2 --profile production
    swissarmycli asg-status my-asg-name --stream
    swissarmycli asg-status my-asg-name -s -i 15 -r eu-central-1
    ```

### `validate [filepath]`

Validates the syntax and structure of YAML configuration files (e.g., Kubernetes manifests, Helm charts).

*   **Syntax:** `swissarmycli validate <filepath>`
*   **Arguments:**
    *   `filepath`: Path to the file to be validated.
*   **Example:**
    ```bash
    swissarmycli validate ./path/to/your/kubernetes-deployment.yaml
    ```

### `reveal-secret [secret-name]`

Finds, decodes, and displays Kubernetes secrets. If no namespace is provided, searches across all namespaces. When multiple secrets with the same name exist, prompts for selection.

*   **Syntax:** `swissarmycli reveal-secret <secret-name> [flags]`
*   **Arguments:**
    *   `secret-name`: Name of the Kubernetes secret.
*   **Flags:**
    *   `--namespace`, `-n`: Namespace of the secret (optional).
*   **Examples:**
    ```bash
    swissarmycli reveal-secret my-secret
    swissarmycli reveal-secret my-secret -n production
    ```

### `check-cert [secret-name]`

Checks TLS certificate details and expiry dates from Kubernetes secrets. Displays certificate subject, issuer, validity period, DNS names, and warns about expiring or expired certificates.

*   **Syntax:** `swissarmycli check-cert <secret-name> [flags]`
*   **Arguments:**
    *   `secret-name`: Name of the TLS secret.
*   **Flags:**
    *   `--namespace`, `-n`: Namespace of the secret (optional).
*   **Examples:**
    ```bash
    swissarmycli check-cert tls-secret
    swissarmycli check-cert tls-secret -n ingress-nginx
    ```

### `cost-estimate`

Estimates monthly costs for your current Kubernetes cluster by analyzing EC2 instances, EBS volumes, and load balancers. Uses pricing data from the embedded configuration file.

*   **Syntax:** `swissarmycli cost-estimate`
*   **Example:**
    ```bash
    swissarmycli cost-estimate
    ```
*   **Output includes:**
    *   EC2 instance types and counts with hourly/monthly costs
    *   EBS volume types and total storage with monthly costs
    *   Load balancer types and counts with hourly/monthly costs
    *   Total estimated monthly cost

**Note:** Pricing data is embedded in the binary from `internal/k8s/cost-estimate.json`. Update this file with current AWS pricing before building to ensure accurate estimates.

## Configuration

### Cost Estimation Pricing

To update pricing data for cost estimation:

1. Edit `internal/k8s/cost-estimate.json` with current AWS pricing
2. Rebuild the binary: `make build`

The JSON file contains pricing for:
- `ec2_pricing`: Hourly rates for EC2 instance types
- `ebs_pricing`: Monthly rates per GB for EBS volume types
- `lb_pricing`: Hourly rates for load balancer types

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

[Add your license information here]
