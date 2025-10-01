package main

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

type container struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	State string   `json:"State"`
}

func listDockerContainers() ([]Sandbox, error) {
	slog.Debug("Listing Docker containers", "socket", *dockerSocketPath)
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (net.Conn, error) {
				return net.Dial("unix", *dockerSocketPath)
			},
		},
	}

	// Inspect all containers.
	resp, err := httpClient.Get("http://localhost/containers/json?all=1")
	if err != nil {
		slog.Error("Failed to get Docker containers", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Parse the response.
	var containers []container
	err = json.NewDecoder(resp.Body).Decode(&containers)
	if err != nil {
		slog.Error("Failed to decode Docker response", "error", err)
		return nil, err
	}

	// Extract running containers.
	var controlGroups []Sandbox
	for _, c := range containers {
		if c.State == "running" {
			slog.Debug("Found Docker container", "id", c.ID, "names", c.Names, "state", c.State)

			// Use the first name without the leading slash as the container name.
			controlGroups = append(controlGroups, Sandbox{
				ID:        c.ID,
				Container: strings.TrimPrefix(c.Names[0], "/"),
			})
		} else {
			slog.Debug("Skipping non-running Docker container", "id", c.ID, "names", c.Names, "state", c.State)
		}
	}

	return controlGroups, nil
}
