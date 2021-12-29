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

func bootstrapDocker(containerId string) ([]byte, error) {
	cmd := exec.Command("docker", "cp", "-", fmt.Sprintf("%s:/", containerId))
	cmd.Stdin, _ = executables.Open("agent.tar")
	return cmd.CombinedOutput()
}

func bootstrapKubernetes(ns, pod string) ([]byte, error) {
	cmd := exec.Command("kubectl", "exec", "-i", "-n", ns, pod, "--", "tar", "xf", "-", "-C", "/")
	cmd.Stdin, _ = executables.Open("agent.tar")
	return cmd.CombinedOutput()
}

func Bootstrap(k8s bool, id string) ([]byte, error) {
	if k8s {
		splits := strings.SplitN(id, "/", 2)
		if len(splits) != 2 {
			return nil, errors.New("invalid kubernetes pod id format ({namespace}/{pod_name}")
		}
		return bootstrapKubernetes(splits[0], splits[1])
	}
	return bootstrapDocker(id)
}
