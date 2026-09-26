// Package compose recreates services with the Docker Compose CLI. It finds the project's files
// through the labels Compose puts on every container it creates, so only the project name is needed.
package compose

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Labels Docker Compose puts on its containers; list values are comma-separated.
const (
	projectLabel     = "com.docker.compose.project"
	workingDirLabel  = "com.docker.compose.project.working_dir"
	configFilesLabel = "com.docker.compose.project.config_files"
	envFilesLabel    = "com.docker.compose.project.environment_file"
)

// Recreating stops and starts containers and may pull images, so it can take minutes.
const timeout = 5 * time.Minute

// Compose errors come last, so a long message keeps its end.
const messageLimit = 2 << 10

// Recreate force-recreates services of project, or the whole project when services is empty.
// Compose re-reads every env_file while doing it, so the new values reach the containers.
// The compose files must be mounted in Docu-UI at the same paths as on the host.
func Recreate(ctx context.Context, project string, services []string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	listing, err := run(ctx, "ps", "--all", "--filter", "label="+projectLabel+"="+project, "--format",
		`{{.Label "`+workingDirLabel+`"}}|{{.Label "`+configFilesLabel+`"}}|{{.Label "`+envFilesLabel+`"}}`)
	if err != nil {
		return err
	}
	firstContainer, _, _ := strings.Cut(strings.TrimSpace(listing), "\n")
	if firstContainer == "" {
		return fmt.Errorf("no container of project %q; start it once with docker compose so Docu-UI can find its files", project)
	}
	// The format above always gives three fields.
	labels := strings.Split(firstContainer, "|")
	arguments := []string{"compose", "--project-name", project, "--project-directory", labels[0]}
	for _, configFile := range splitList(labels[1]) {
		arguments = append(arguments, "--file", configFile)
	}
	for _, envFile := range splitList(labels[2]) {
		arguments = append(arguments, "--env-file", envFile)
	}
	arguments = append(arguments, "up", "--detach", "--force-recreate", "--no-deps")
	_, err = run(ctx, append(arguments, services...)...)
	return err
}

func splitList(labelValue string) []string {
	if labelValue == "" {
		return nil
	}
	return strings.Split(labelValue, ",")
}

// run calls the docker CLI and returns its output, or an error carrying what it printed.
func run(ctx context.Context, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	var output, messages bytes.Buffer
	command.Stdout, command.Stderr = &output, &messages
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(messages.String())
		if message == "" {
			message = err.Error()
		}
		if len(message) > messageLimit {
			message = "…" + message[len(message)-messageLimit:]
		}
		return "", fmt.Errorf("docker %s failed: %s", arguments[0], message)
	}
	return output.String(), nil
}
