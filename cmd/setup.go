package cmd

import (
	"fmt"
	"strings"

	"GoDistribute/config"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var configFile string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup servers for GoDistribute",
	Run:   runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVar(&configFile, "config", "", "Config file (required)")
	setupCmd.MarkFlagRequired("config")
}

func checkPrerequisites(client *ssh.Client, hostname string) (bool, error) {
	checks := []struct {
		cmd     string
		errMsg  string
		success string
	}{
		{"hostname", "Failed to get hostname", "Hostname: %s"},
		{"which curl", "curl is not installed", "curl is installed: %s"},
		{"sudo -n true", "Failed to run sudo", "Sudo permission granted"},
	}

	for _, check := range checks {
		output, stderr, err := runSSHCommand(client, check.cmd)
		if err != nil {
			return false, fmt.Errorf("%s on %s: %v\nstderr: %s", check.errMsg, hostname, err, stderr)
		}
		if strings.Contains(check.success, "%s") {
			PrintAppOutputf("\r\033[K"+check.success+" on %s\r", strings.TrimSpace(output), hostname)
		} else {
			PrintAppOutputf("\r\033[K"+check.success+" on %s\r", hostname)
		}
	}
	return true, nil
}

func setupPodman(sshClient *ssh.Client, server config.Server) error {
	podmanURL := "https://github.com/mgoltzsche/podman-static/releases/download/v5.2.2/podman-linux-amd64.tar.gz"
	commands := []struct {
		cmd     string
		success string
	}{
		{fmt.Sprintf("curl -fsSL -o podman.tar.gz %s", podmanURL), "Successfully downloaded Podman"},
		{"mkdir -p ~/podman-bin && tar -xzf podman.tar.gz -C ~/podman-bin", "Successfully untarred Podman"},
	}

	for _, command := range commands {
		if _, stderr, err := runSSHCommand(sshClient, command.cmd); err != nil {
			return fmt.Errorf("failed to %s on %s: %v\nstderr: %s", command.success, server.Hostname, err, stderr)
		}
		PrintAppOutputf("\r\033[K"+"%s on %s\r", command.success, server.Hostname)
	}
	return nil
}

func findPodmanBinaries(sshClient *ssh.Client, hostname string) (string, string, error) {
	binaries := []struct {
		name string
		cmd  string
	}{
		{"conmon", "find ~/podman-bin -name conmon"},
		{"crun", "find ~/podman-bin -name crun"},
	}

	var paths []string
	for _, binary := range binaries {
		output, _, err := runSSHCommand(sshClient, binary.cmd)
		if err != nil {
			return "", "", fmt.Errorf("failed to find %s on %s: %v", binary.name, hostname, err)
		}
		path := strings.TrimSpace(output)
		PrintAppOutputf("\r\033[K"+"%s location on %s: %s\r", binary.name, hostname, path)
		paths = append(paths, path)
	}
	return paths[0], paths[1], nil
}

func setupConfigurations(sshClient *ssh.Client, hostname, homeDir string) error {
	conmonPath, crunPath, err := findPodmanBinaries(sshClient, hostname)
	if err != nil {
		return fmt.Errorf("failed to find Podman binaries on %s: %v", hostname, err)
	}

	configs := []struct {
		name    string
		content string
	}{
		{"containers.conf", fmt.Sprintf(`
[engine]
runtime = "%s"
conmon_path = ["%s"]
helper_binaries_dir = ["%s/podman-bin/podman-linux-amd64/usr/local/lib/podman"]
`, crunPath, conmonPath, homeDir)},
		{"policy.json", `
{
    "default": [
        {
            "type": "insecureAcceptAnything"
        }
    ],
    "transports":
        {
            "docker-daemon":
                {
                    "": [{"type":"insecureAcceptAnything"}]
                }
        }
}
`},
	}

	for _, config := range configs {
		cmd := fmt.Sprintf(`
mkdir -p %s/.config/containers
cat <<EOT > %s/.config/containers/%s
%s
EOT
`, homeDir, homeDir, config.name, config.content)
		if _, stderr, err := runSSHCommand(sshClient, cmd); err != nil {
			return fmt.Errorf("failed to set up %s on %s: %v\nstderr: %s", config.name, hostname, err, stderr)
		}
		PrintAppOutputf("\r\033[K"+"Set up %s on %s\r", config.name, hostname)
	}
	return nil
}

func testPodmanInstallation(sshClient *ssh.Client, hostname, podmanCmd string) error {

	tests := []struct {
		cmd     string
		success string
	}{
		{fmt.Sprintf("%s --version", podmanCmd), "Podman version"},
		{fmt.Sprintf("%s run --rm docker.io/library/hello-world", podmanCmd), "Successfully ran Hello World container"},
	}

	for _, test := range tests {
		_, stderr, err := runSSHCommand(sshClient, test.cmd)
		if err != nil {
			return fmt.Errorf("failed to %s on %s: %v\nstderr: %s", test.success, hostname, err, stderr)
		}
		PrintAppOutputf("\r\033[K"+"%s on %s\r", test.success, hostname)
	}
	return nil
}

func runSetup(cmd *cobra.Command, args []string) {

	servers, err := config.LoadServers(configFile)
	if err != nil {
		PrintAppErrorfAndExit("error loading server configurations: %v", err)
	}

	// Setup servers
	for _, server := range servers {
		PrintAppOutputf("================================================\n")
		PrintAppOutputf("Setting up server: %s\n", server.Hostname)

		sshClient, err := createSSHClient(server)
		if err != nil {
			PrintAppErrorfAndExit("Failed to connect to %s: %v", server.Hostname, err)
		}
		defer sshClient.Close()

		_, err = checkPrerequisites(sshClient, server.Hostname)
		if err != nil {
			PrintAppErrorfAndExit("Failed to check prerequisites on %s: %v", server.Hostname, err)
		}

		homeDir, err := getHomeDirectory(sshClient, server)
		if err != nil {
			PrintAppErrorfAndExit("Failed to get home directory on %s: %v", server.Hostname, err)
		}

		if err := setupPodman(sshClient, server); err != nil {
			PrintAppErrorfAndExit("Failed to setup Podman on %s: %v", server.Hostname, err)
		}

		if err := setupConfigurations(sshClient, server.Hostname, homeDir); err != nil {
			PrintAppErrorfAndExit("Failed to setup configurations on %s: %v", server.Hostname, err)
		}

		podmanCmd := getPodmanCmd(homeDir)
		if err := testPodmanInstallation(sshClient, server.Hostname, podmanCmd); err != nil {
			PrintAppErrorfAndExit("Failed to test Podman installation on %s: %v", server.Hostname, err)
		}
		PrintAppOutputf("\r\033[K")
		PrintAppOutputf("Setup complete for %s\n", server.Hostname)
		PrintAppOutputf("================================================\n")

	}

	// fmt.Println("Setup complete")
}
