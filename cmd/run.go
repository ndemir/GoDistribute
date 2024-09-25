package cmd

import (
	"GoDistribute/config"
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	command        string
	jobsPerNode    int
	imageTar       string
	showPercentage bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run distributed tasks",
	Long:  `Run distributed tasks across configured servers using a local image.`,
	Run:   runParallelTask,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&command, "command", "", "Command to run (required)")
	runCmd.Flags().IntVar(&jobsPerNode, "jobs-per-node", 1, "Number of jobs per server")
	runCmd.Flags().StringVar(&configFile, "config", "", "Config file (required)")
	runCmd.Flags().StringVar(&imageTar, "image-tar", "", "Local image tar file")
	runCmd.Flags().BoolVar(&showPercentage, "show-percentage", false, "Show job completion percentage")
	runCmd.MarkFlagRequired("command")
	runCmd.MarkFlagRequired("config")
}

func workerRoutine(
	server config.Server,
	commandTemplate string,
	imageName string,
	jobs <-chan string,
	taskStdoutChan chan<- string,
	taskStderrChan chan<- string,
	appStderrChan chan<- string,
	// wg *sync.WaitGroup,
) {

	// defer wg.Done()

	// PrintAppOutputf("DEBUG: workerRoutine started for %s\n", server.Hostname)

	sshClient, err := createSSHClient(server)
	if err != nil {
		appStderrChan <- fmt.Sprintf("Error connecting to %s: %v", server.Hostname, err)
		return
	}
	defer sshClient.Close()

	for job := range jobs {
		// PrintAppOutputf("DEBUG: jobChan -> %s on %s\n", job, server.Hostname)
		command := strings.ReplaceAll(commandTemplate, "{}", job)
		if imageTar != "" {
			homeDir, err := getHomeDirectory(sshClient, server)
			if err != nil {
				appStderrChan <- fmt.Sprintf(
					"Error getting home directory for %s: %v",
					server.Hostname,
					err,
				)
				continue
			}
			podmanCmd := getPodmanCmd(homeDir)
			command = fmt.Sprintf("%s run --rm %s:%s %s", podmanCmd, imageName, imageName, command)
		}
		stdoutOutput, stderrOutput, err := runSSHCommand(sshClient, command)
		if err != nil {
			taskStderrChan <- stderrOutput
		} else {
			taskStdoutChan <- stdoutOutput
		}
	}
	// PrintAppOutputf("DEBUG: workerRoutine done for %s\n", server.Hostname)
}

func setupContainer(server config.Server, containerTar string, imageName string) error {
	authMethod, err := publicKeyFile(server.SSHKeyFile)
	if err != nil {
		return fmt.Errorf("failed to get public key file: %v", err)
	}
	sshConfig := &ssh.ClientConfig{
		User: server.Username,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.Hostname, server.Port), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", server.Hostname, err)
	}
	defer client.Close()

	// Transfer container tar
	if err := transferFile(client, containerTar, filepath.Base(containerTar)); err != nil {
		return fmt.Errorf("failed to transfer container: %v", err)
	}

	// Get home directory
	homeDir, err := getHomeDirectory(client, server)
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	// Load container
	podmanCmd := getPodmanCmd(homeDir)
	tarPath := "/tmp/" + filepath.Base(containerTar)
	loadCmd := fmt.Sprintf("%s load -i %s", podmanCmd, tarPath)
	stdout, stderr, err := runSSHCommand(client, loadCmd)
	if err != nil {
		return fmt.Errorf(
			"failed to load container on %s: %v\nstderr: %s",
			server.Hostname,
			err,
			stderr,
		)
	}
	loadedImageLine := strings.Split(stdout, "\n")[0]
	loadedImage := strings.Split(loadedImageLine, " ")[2]

	tagCmd := fmt.Sprintf("%s tag %s %s:%s", podmanCmd, loadedImage, imageName, imageName)
	_, stderr, err = runSSHCommand(client, tagCmd)
	if err != nil {
		return fmt.Errorf(
			"failed to tag container on %s: %v\nstderr: %s",
			server.Hostname,
			err,
			stderr,
		)
	}

	return nil
}

func runParallelTask(cmd *cobra.Command, args []string) {
	servers, err := config.LoadServers(configFile)
	if err != nil {
		PrintAppErrorfAndExit("Error loading server configurations: %v", err)
	}

	nServers := len(servers)
	copyCompleted := 0
	copyCompletedChan := make(chan bool, nServers)

	imageName := ""
	if imageTar != "" {
		imageName = fmt.Sprintf("task-%d", rand.Intn(1000))
		semaphore := make(chan struct{}, 2) // Limit to 2 concurrent operations

		for _, server := range servers {
			semaphore <- struct{}{} // Acquire semaphore
			go func(s config.Server) {
				defer func() {
					<-semaphore // Release semaphore
				}()

				if err := setupContainer(s, imageTar, imageName); err != nil {
					PrintAppErrorfAndExit("Error setting up image on %s: %v", s.Hostname, err)
				}
				copyCompletedChan <- true
			}(server)
		}

		for i := 0; i < nServers; i++ {
			<-copyCompletedChan
			copyCompleted++
			PrintAppOutputf("\r%d/%d nodes completed image setup", copyCompleted, nServers)
		}
		PrintAppOutputf("\n")
	}

	jobChan := make(chan string, 100)

	taskStdoutChan := make(chan string, 100)
	taskStderrChan := make(chan string, 100)
	appStdoutChan := make(chan string, 100)
	appStderrChan := make(chan string, 100)

	var producer_wg sync.WaitGroup
	var consumer_wg sync.WaitGroup

	for _, server := range servers {
		// go func(s config.Server) {
		for i := 0; i < jobsPerNode; i++ {
			producer_wg.Add(1)
			go func() {
				defer producer_wg.Done()
				workerRoutine(
					server,
					command,
					imageName,
					jobChan,
					taskStdoutChan,
					taskStderrChan,
					appStderrChan,
				)

			}()
		}
		// }(server)
	}

	// Print app output
	consumer_wg.Add(1)
	go func() {
		defer consumer_wg.Done()
		for o := range appStdoutChan {
			PrintAppOutputf("%s", o)
		}
	}()

	// Print app error output
	consumer_wg.Add(1)
	go func() {
		defer consumer_wg.Done()
		for o := range appStderrChan {
			PrintAppErrorfAndExit("%s", o)
		}
	}()

	totalJobs := 0
	completedJobs := 0
	var mu sync.Mutex

	// Print task output
	consumer_wg.Add(1)
	go func() {
		defer consumer_wg.Done()
		for o := range taskStdoutChan {
			PrintTaskOutputf("%s", o)
			mu.Lock()
			completedJobs++
			message := ""
			if showPercentage {
				percentage := float64(completedJobs) / float64(totalJobs) * 100
				message = fmt.Sprintf("\r%.2f%% (%d/%d) jobs completed.", percentage, completedJobs, totalJobs)
			} else {
				message = fmt.Sprintf("\r%d jobs completed.", completedJobs)
			}
			PrintAppOutputf("%s", message)
			mu.Unlock()
		}
		PrintAppOutputf("\n")

		PrintAppOutputf("Total completed: %d\n", completedJobs)
	}()

	// Print task error output
	consumer_wg.Add(1)
	go func() {
		defer consumer_wg.Done()
		for o := range taskStderrChan {
			mu.Lock()
			completedJobs++
			PrintTaskErrorf("%s", o)
			mu.Unlock()
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	if showPercentage {
		// Read all jobs into memory if percentage is requested
		var jobs []string
		for scanner.Scan() {
			jobs = append(jobs, scanner.Text())
		}
		totalJobs = len(jobs)
		PrintAppOutputf("Total jobs: %d\n", totalJobs)
		for _, job := range jobs {
			// PrintAppOutputf("DEBUG: jobChan <- %s\n", job)
			jobChan <- job
		}
		close(jobChan)

	} else {

		var input_wg sync.WaitGroup
		input_wg.Add(1)
		go func() {
			defer input_wg.Done()
			for scanner.Scan() {
				totalJobs++
				jobChan <- scanner.Text()
			}
			// close(jobChan)
		}()
		go func() {
			input_wg.Wait()
			close(jobChan)
		}()
	}
	producer_wg.Wait()
	// close(jobChan)
	close(taskStdoutChan)
	close(taskStderrChan)
	close(appStdoutChan)
	close(appStderrChan)
	consumer_wg.Wait()
	// time.Sleep(1 * time.Second)

}
