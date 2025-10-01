# cgroup-container-exporter

Exports Linux cgroup metrics for containers in a format compatible with Prometheus scraping.

Supports both Kubernetes (kind) and Docker (docker compose) environments.

For details on the exported cgroup v2 metrics, refer to [metrics.go](./metrics.go) and the official [cgroup v2 Kernel documentation](https://docs.kernel.org/admin-guide/cgroup-v2.html).

## Command Line Arguments

| Argument           | Default                           | Description                                  |
| ------------------ | --------------------------------- | -------------------------------------------- |
| `-addr`            | `:8080`                           | Address to listen on for HTTP requests       |
| `-host-path`       | `/host`                           | Path where host filesystem is mounted        |
| `-scrape-interval` | `1s`                              | Scrape interval for metrics                  |
| `-docker-sock`     | `/var/run/docker.sock`            | Path to Docker socket                        |
| `-containerd-sock` | `/run/containerd/containerd.sock` | Path to containerd socket                    |
| `-mode`            | `kubernetes`                      | Container runtime mode: docker or kubernetes |

Access the metrics at `http://<host>:8080/metrics`.

## Examples

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cgroup-container-exporter
spec:
  selector:
    matchLabels:
      app: cgroup-container-exporter
  template:
    metadata:
      labels:
        app: cgroup-container-exporter
    spec:
      containers:
        - name: exporter
          image: ghcr.io/tsaarni/cgroup-container-exporter:latest
          command:
            - /cgroup-container-exporter
          args:
            - "-host-path=/host"
            - "-mode=kubernetes"
          ports:
            - containerPort: 8080
          volumeMounts:
            - name: host-cgroup
              mountPath: /host/sys/fs/cgroup
              readOnly: true
            - name: containerd-sock
              mountPath: /run/containerd/containerd.sock
              readOnly: true
      volumes:
        - name: host-cgroup
          hostPath:
            path: /sys/fs/cgroup
            type: Directory
        - name: containerd-sock
          hostPath:
            path: /run/containerd/containerd.sock
            type: Socket
---
apiVersion: v1
kind: Service
metadata:
    name: cgroup-container-exporter
spec:
    ports:
        - port: 8080
    selector:
        app: cgroup-container-exporter
```

### Docker Compose

```yaml
services:
  cgroup-container-exporter:
    image: ghcr.io/tsaarni/cgroup-container-exporter:latest
    entrypoint:
      - /cgroup-container-exporter
      - --host-path=/host
      - --mode=docker
    volumes:
      - /sys/fs/cgroup:/host/sys/fs/cgroup:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
```
