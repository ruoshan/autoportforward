package bootstrap

import (
	"embed"
	"fmt"
	"os/exec"
)

//go:embed agent.tar
var executables embed.FS

func bootstrapDocker(containerId string) ([]byte, error) {
	cmd := exec.Command("docker", "cp", "-", fmt.Sprintf("%s:/", containerId))
	cmd.Stdin, _ = executables.Open("agent.tar")
	return cmd.CombinedOutput()
}

func Bootstrap(k8s bool, id string) ([]byte, error) {
	if !k8s {
		return bootstrapDocker(id)
	}
	return nil, nil
}
