package bootstrap

import (
	"embed"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed agent.tar
var executables embed.FS

type RTType uint8

const (
	DOCKER RTType = iota
	KUBERNETES
	PODMAN
)

func bootstrapDocker(containerId string) ([]byte, error) {
	cmd := exec.Command("docker", "cp", "-", fmt.Sprintf("%s:/", containerId))
	cmd.Stdin, _ = executables.Open("agent.tar")
	return cmd.CombinedOutput()
}

// NB: Due to the limitation of the `kubectl cp/exec`, the target container image must have
// `tar` in it.
func bootstrapKubernetes(ns, pod string) ([]byte, error) {
	cmd := exec.Command("kubectl", "exec", "-i", "-n", ns, pod, "--", "tar", "xf", "-", "-C", "/")
	cmd.Stdin, _ = executables.Open("agent.tar")
	return cmd.CombinedOutput()
}

func bootstrapPodman(containerId string) ([]byte, error) {
	cmd := exec.Command("podman", "cp", "-", fmt.Sprintf("%s:/", containerId))
	cmd.Stdin, _ = executables.Open("agent.tar")
	return cmd.CombinedOutput()
}

func Bootstrap(rt RTType, id string) ([]byte, error) {
	switch rt {
	case DOCKER:
		return bootstrapDocker(id)
	case KUBERNETES:
		splits := strings.SplitN(id, "/", 2)
		if len(splits) != 2 {
			return nil, errors.New("invalid kubernetes pod id format ({namespace}/{pod_name}")
		}
		return bootstrapKubernetes(splits[0], splits[1])
	case PODMAN:
		return bootstrapPodman(id)
	default:
		panic("Unknown runtime type")
	}
}
