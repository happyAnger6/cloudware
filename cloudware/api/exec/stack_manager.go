package exec

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"runtime"

	"cloudware/cloudware/api"
)

// StackManager represents a service for managing stacks.
type StackManager struct {
	binaryPath string
}

// NewStackManager initializes a new StackManager service.
func NewStackManager(binaryPath string) *StackManager {
	return &StackManager{
		binaryPath: binaryPath,
	}
}

// Login executes the docker login command against a list of registries (including DockerHub).
func (manager *StackManager) Login(dockerhub *api.DockerHub, registries []api.Registry, endpoint *api.Endpoint) error {
	command, args := prepareDockerCommandAndArgs(manager.binaryPath, endpoint)
	for _, registry := range registries {
		if registry.Authentication {
			registryArgs := append(args, "login", "--username", registry.Username, "--password", registry.Password, registry.URL)
			err := runCommandAndCaptureStdErr(command, registryArgs, nil)
			if err != nil {
				return err
			}
		}
	}

	if dockerhub.Authentication {
		dockerhubArgs := append(args, "login", "--username", dockerhub.Username, "--password", dockerhub.Password)
		err := runCommandAndCaptureStdErr(command, dockerhubArgs, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// Logout executes the docker logout command.
func (manager *StackManager) Logout(endpoint *api.Endpoint) error {
	command, args := prepareDockerCommandAndArgs(manager.binaryPath, endpoint)
	args = append(args, "logout")
	return runCommandAndCaptureStdErr(command, args, nil)
}

// Deploy executes the docker stack deploy command.
func (manager *StackManager) Deploy(stack *api.Stack, endpoint *api.Endpoint) error {
	stackFilePath := path.Join(stack.ProjectPath, stack.EntryPoint)
	command, args := prepareDockerCommandAndArgs(manager.binaryPath, endpoint)
	args = append(args, "stack", "deploy", "--with-registry-auth", "--compose-file", stackFilePath, stack.Name)

	env := make([]string, 0)
	for _, envvar := range stack.Env {
		env = append(env, envvar.Name+"="+envvar.Value)
	}

	return runCommandAndCaptureStdErr(command, args, env)
}

// Remove executes the docker stack rm command.
func (manager *StackManager) Remove(stack *api.Stack, endpoint *api.Endpoint) error {
	command, args := prepareDockerCommandAndArgs(manager.binaryPath, endpoint)
	args = append(args, "stack", "rm", stack.Name)
	return runCommandAndCaptureStdErr(command, args, nil)
}

func runCommandAndCaptureStdErr(command string, args []string, env []string) error {
	var stderr bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stderr = &stderr

	if env != nil {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, env...)
	}

	err := cmd.Run()
	if err != nil {
		return api.Error(stderr.String())
	}

	return nil
}

func prepareDockerCommandAndArgs(binaryPath string, endpoint *api.Endpoint) (string, []string) {
	// Assume Linux as a default
	command := path.Join(binaryPath, "docker")

	if runtime.GOOS == "windows" {
		command = path.Join(binaryPath, "docker.exe")
	}

	args := make([]string, 0)
	args = append(args, "-H", endpoint.URL)

	if endpoint.TLSConfig.TLS {
		args = append(args, "--tls")

		if !endpoint.TLSConfig.TLSSkipVerify {
			args = append(args, "--tlsverify", "--tlscacert", endpoint.TLSConfig.TLSCACertPath)
		}

		if endpoint.TLSConfig.TLSCertPath != "" && endpoint.TLSConfig.TLSKeyPath != "" {
			args = append(args, "--tlscert", endpoint.TLSConfig.TLSCertPath, "--tlskey", endpoint.TLSConfig.TLSKeyPath)
		}
	}

	return command, args
}
