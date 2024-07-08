# K8s Resource Monitor

K8s Resource Monitor is a Go-based microservice that monitors custom resources deployed in a Kubernetes cluster. It continuously verifies the status of these resources and updates their overall health status based on predefined criteria.

## Features

- Monitors any custom resource in the Kubernetes cluster.
- Performs health checks on Pods, Jobs, PersistentVolumes (PVs), and PersistentVolumeClaims (PVCs).
- Provides an HTTP API to retrieve health status and reset resource status.
- Configurable health check intervals and thresholds.
- Rate limiting to prevent overloading the Kubernetes API server.
- Initial delay configuration for health checks.

## Prerequisites

- Kubernetes cluster
- Go 1.15+ (for building the microservice)
- kubectl (for deploying the microservice)

## Installation

### Building from Source

1. Clone the repository:

    ```sh
    git clone https://github.com/your-github-username/k8s-resource-monitor.git
    cd k8s-resource-monitor
    ```

2. Build the microservice:

    ```sh
    go build -o k8s-resource-monitor main.go
    ```

3. Create a Docker image:

    ```sh
    docker build -t your-docker-repo/k8s-resource-monitor:latest .
    ```

4. Push the Docker image to your Docker registry:

    ```sh
    docker push your-docker-repo/k8s-resource-monitor:latest
    ```

### Deploying to Kubernetes

1. Apply the Kubernetes manifests:

    ```sh
    kubectl apply -f k8s/deployment.yaml
    ```

2. Check the status of the deployment:

    ```sh
    kubectl get pods -l app=k8s-resource-monitor
    ```

## Configuration

You can configure K8s Resource Monitor using environment variables:

- `CHECK_INTERVAL`: Interval between health checks (default: 25s)
- `CONSEC_HEALTHY`: Number of consecutive healthy checks required to mark a resource as ready (default: 5)
- `CONSEC_FAILED`: Number of consecutive failed checks required to mark a resource as failed (default: 3)
- `LIMITER_RATE`: Rate limit for health checks (default: 10)
- `LIMITER_BURST`: Burst limit for health checks (default: 20)
- `INCREASE_INTERVAL_VALUE`: Interval increase value for failed checks (default: 20s)
- `READY_CHECK_INTERVAL`: Interval to recheck ready resources (default: 60s)
- `INITIAL_DELAY`: Initial delay before starting health checks (default: 10s)

## Usage

### API Endpoints

- **Health Check Endpoint**: `/healthz`
  - **Method**: GET
  - **Response**: 200 OK

- **Get Resource Health Endpoint**: `/health/{crdGroup}/{crdVersion}/{crdPlural}/{namespace}/{name}`
  - **Method**: GET
  - **Response**: JSON object with resource status

- **Reset Resource Status Endpoint**: `/reset/{crdGroup}/{crdVersion}/{crdPlural}/{namespace}/{name}`
  - **Method**: POST
  - **Response**: 200 OK if reset successfully

### Example Requests

1. **Check Microservice Health**

    ```sh
    curl -X GET http://<service-ip>:8080/healthz
    ```

2. **Get Custom Resource Health**

    ```sh
    curl -X GET http://<service-ip>:8080/health/<crdGroup>/<crdVersion>/<crdPlural>/<namespace>/<name>
    ```

3. **Reset Custom Resource Status**

    ```sh
    curl -X POST http://<service-ip>:8080/reset/<crdGroup>/<crdVersion>/<crdPlural>/<namespace>/<name>
    ```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any enhancements or bug fixes.

## License

This project is licensed under the Apache 2.0 License. See the [LICENSE](LICENSE) file for details.

## Acknowledgements

Special thanks to the Kubernetes and Go communities for their excellent tools and documentation.
