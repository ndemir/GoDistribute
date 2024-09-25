package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"GoDistribute/config"

	"golang.org/x/crypto/ssh"
)

func createSSHClient(server config.Server) (*ssh.Client, error) {
	authMethod, err := publicKeyFile(server.SSHKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key file: %v", err)
	}
	sshConfig := &ssh.ClientConfig{
		User: server.Username,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.Hostname, server.Port), sshConfig)
}

func publicKeyFile(file string) (ssh.AuthMethod, error) {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key file: %v", err)
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %v", err)
	}

	return ssh.PublicKeys(key), nil
}

func runSSHCommand(sshClient *ssh.Client, command string) (string, string, error) {
	session, err := sshClient.NewSession()

	if err != nil {
		return "", "", fmt.Errorf("failed to create session on %s: %v", sshClient.RemoteAddr().String(), err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	return stdout.String(), stderr.String(), err
}

func getHomeDirectory(sshClient *ssh.Client, server config.Server) (string, error) {
	if server.HomeDir != "" {
		return server.HomeDir, nil
	}
	homeDir, _, err := runSSHCommand(sshClient, "echo $HOME")
	if err != nil {
		return "", err
	}
	server.HomeDir = strings.TrimSpace(homeDir)
	return server.HomeDir, nil
}

func getPodmanCmd(homeDir string) string {
	additionalPath := fmt.Sprintf(
		"%s/podman-bin/podman-linux-amd64/usr/local/lib/podman:%s/podman-bin/podman-linux-amd64/usr/local/bin",
		homeDir,
		homeDir,
	)
	podmanCmd := fmt.Sprintf(
		"PATH=$PATH:%s CONTAINERS_CONF=%s/.config/containers/containers.conf sudo -E ~/podman-bin/podman-linux-amd64/usr/local/bin/podman",
		additionalPath,
		homeDir,
	)
	return podmanCmd
}

func transferFile(client *ssh.Client, localPath, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %v", err)
	}

	go func() {
		w, _ := session.StdinPipe()
		fmt.Fprintf(w, "C%#o %d %s\n", stat.Mode().Perm(), stat.Size(), remotePath)
		io.Copy(w, f)
		fmt.Fprint(w, "\x00")
		w.Close()
	}()

	if err := session.Run("/usr/bin/scp -qt /tmp"); err != nil {
		return fmt.Errorf("failed to run scp: %v", err)
	}

	return nil
}
