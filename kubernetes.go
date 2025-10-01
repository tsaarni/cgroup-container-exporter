package main

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func listKubernetesPods() ([]Sandbox, error) {
	slog.Debug("Listing Kubernetes pods", "socket", *containerdSocketPath)
	conn, err := grpc.NewClient(fmt.Sprintf("unix://%s", *containerdSocketPath), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("Failed to connect to containerd socket", "error", err)
		return nil, err
	}
	defer conn.Close()

	client := runtimeapi.NewRuntimeServiceClient(conn)
	resp, err := client.ListPodSandbox(context.Background(), &runtimeapi.ListPodSandboxRequest{})
	if err != nil {
		slog.Error("Failed to list pod sandboxes", "error", err)
		return nil, err
	}

	var controlGroups []Sandbox
	for _, pod := range resp.Items {
		if pod.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			slog.Debug("Found running pod", "name", pod.Metadata.Name, "uid", pod.Metadata.Uid, "namespace", pod.Metadata.Namespace, "id", pod.Id)
			controlGroups = append(controlGroups, Sandbox{
				ID:        pod.Id,
				Namespace: pod.Metadata.Namespace,
				Pod:       pod.Metadata.Name,
			})
		} else {
			slog.Debug("Skipping non-running pod", "name", pod.Metadata.Name, "uid", pod.Metadata.Uid, "namespace", pod.Metadata.Namespace, "id", pod.Id, "state", pod.State)
		}

		containers, err := client.ListContainers(context.Background(), &runtimeapi.ListContainersRequest{
			Filter: &runtimeapi.ContainerFilter{
				PodSandboxId: pod.Id,
			},
		})
		if err != nil {
			slog.Error("Failed to list containers for pod", "pod", pod.Id, "error", err)
			continue
		}

		for _, container := range containers.Containers {
			slog.Debug("Found container in pod", "pod", pod.Id, "container", container.Metadata.Name, "id", container.Id)
			if container.State == runtimeapi.ContainerState_CONTAINER_RUNNING {
				controlGroups = append(controlGroups, Sandbox{
					ID:        container.Id,
					Container: container.Metadata.Name,
					Namespace: pod.Metadata.Namespace,
					Pod:       pod.Metadata.Name,
				})
			} else {
				slog.Debug("Skipping non-running container in pod", "pod", pod.Id, "container", container.Metadata.Name, "id", container.Id, "state", container.State)
			}
		}
	}
	return controlGroups, nil
}
