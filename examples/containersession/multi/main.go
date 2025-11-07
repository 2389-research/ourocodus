package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/containersession/helpers"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== ContainerSession Multi-Session Example ===")
	fmt.Println("Demonstrates concurrent sessions coordinating via shared workspace.")

	// 1. Connect to Docker
	fmt.Println("Step 1: Connecting to Docker...")
	dockerClient, err := helpers.CreateDockerClient(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v\n", err)
	}
	defer dockerClient.Close()
	fmt.Println("✓ Connected")

	// 2. Create shared workspace base
	baseWorkspace := "./workspaces/multi-session"
	sharedDir := filepath.Join(baseWorkspace, "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		log.Fatalf("Failed to create shared directory: %v\n", err)
	}
	defer os.RemoveAll(baseWorkspace)

	fmt.Printf("Step 2: Created shared workspace: %s\n\n", sharedDir)

	// 3. Create Manager (shared by all sessions)
	fmt.Println("Step 3: Creating Manager for all sessions...")
	manager := containersession.NewManager(
		dockerClient,
		&helpers.UUIDGenerator{},
		&helpers.SystemClock{},
		&helpers.StdLogger{Logger: log.New(os.Stdout, "[manager] ", 0)},
		baseWorkspace,
	)
	fmt.Println("✓ Manager created")

	// 4. Launch 3 concurrent sessions
	fmt.Println("Step 4: Launching 3 concurrent container sessions...")
	var wg sync.WaitGroup

	// Session A: Producer (writes tasks)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runProducer(ctx, dockerClient, manager, sharedDir)
	}()

	// Session B: Consumer (reads and processes tasks)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runConsumer(ctx, dockerClient, manager, sharedDir)
	}()

	// Session C: Monitor (watches progress)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runMonitor(ctx, dockerClient, manager, sharedDir)
	}()

	fmt.Println("✓ All sessions launched")

	// Wait for all sessions to complete
	fmt.Println("Step 5: Waiting for sessions to complete...")
	wg.Wait()
	fmt.Println("✓ All sessions completed")

	// 6. Show final results
	fmt.Println("Step 6: Final workspace state...")
	showWorkspaceContents(sharedDir)

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  • Multiple sessions can run concurrently")
	fmt.Println("  • Shared workspace enables inter-session communication")
	fmt.Println("  • File-based coordination is simple and reliable")
	fmt.Println("  • Each session has its own isolated container")
}

// runProducer creates tasks for the consumer to process
func runProducer(ctx context.Context, dockerClient *client.Client, manager *containersession.Manager, sharedDir string) {
	fmt.Println("[Producer] Starting...")

	// Create session
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "20"})
	if err != nil {
		log.Printf("[Producer] Failed to create session: %v\n", err)
		return
	}

	// Create symlink to shared directory in this session's workspace
	sessionShared := filepath.Join(session.WorkspacePath(), "shared")
	if err := os.Symlink(sharedDir, sessionShared); err != nil {
		log.Printf("[Producer] Failed to link shared dir: %v\n", err)
		return
	}

	if err := manager.StartContainerSession(ctx, session.ID()); err != nil {
		log.Printf("[Producer] Failed to start session: %v\n", err)
		return
	}

	fmt.Printf("[Producer] Session %s started\n", session.ID()[:8])

	// Produce 3 tasks
	tasks := []string{
		"Process data file A",
		"Generate report B",
		"Validate results C",
	}

	for i, task := range tasks {
		taskFile := filepath.Join(sharedDir, fmt.Sprintf("task-%d.txt", i+1))
		if err := os.WriteFile(taskFile, []byte(task), 0o644); err != nil {
			log.Printf("[Producer] Failed to write task %d: %v\n", i+1, err)
			continue
		}
		fmt.Printf("[Producer] Created task-%d.txt: %s\n", i+1, task)
		time.Sleep(500 * time.Millisecond)
	}

	// Signal completion
	doneFile := filepath.Join(sharedDir, "producer-done.flag")
	if err := os.WriteFile(doneFile, []byte("complete"), 0o644); err != nil {
		log.Printf("[Producer] Failed to write done flag: %v\n", err)
	}

	fmt.Println("[Producer] All tasks created, signaling completion")

	// Cleanup
	if err := manager.StopContainerSession(ctx, session.ID()); err != nil {
		log.Printf("[Producer] Failed to stop session: %v\n", err)
	}
	fmt.Println("[Producer] Session stopped")

	// Remove container
	if err := dockerClient.ContainerRemove(ctx, session.ContainerID(), container.RemoveOptions{}); err != nil {
		log.Printf("[Producer] Warning: Failed to remove container: %v\n", err)
	} else {
		fmt.Println("[Producer] Container removed")
	}
}

// runConsumer processes tasks created by the producer
func runConsumer(ctx context.Context, dockerClient *client.Client, manager *containersession.Manager, sharedDir string) {
	fmt.Println("[Consumer] Starting...")

	// Create session
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "20"})
	if err != nil {
		log.Printf("[Consumer] Failed to create session: %v\n", err)
		return
	}

	// Link to shared directory
	sessionShared := filepath.Join(session.WorkspacePath(), "shared")
	if err := os.Symlink(sharedDir, sessionShared); err != nil {
		log.Printf("[Consumer] Failed to link shared dir: %v\n", err)
		return
	}

	if err := manager.StartContainerSession(ctx, session.ID()); err != nil {
		log.Printf("[Consumer] Failed to start session: %v\n", err)
		return
	}

	fmt.Printf("[Consumer] Session %s started\n", session.ID()[:8])

	// Wait for tasks and process them
	processedCount := 0
	timeout := time.After(10 * time.Second)

	for processedCount < 3 {
		select {
		case <-timeout:
			fmt.Println("[Consumer] Timeout waiting for tasks")
			goto cleanup
		default:
			// Check for tasks
			for i := 1; i <= 3; i++ {
				taskFile := filepath.Join(sharedDir, fmt.Sprintf("task-%d.txt", i))
				resultFile := filepath.Join(sharedDir, fmt.Sprintf("result-%d.txt", i))

				// Skip if already processed
				if _, err := os.Stat(resultFile); err == nil {
					continue
				}

				// Check if task exists
				taskData, err := os.ReadFile(taskFile)
				if err != nil {
					continue
				}

				// Process the task
				fmt.Printf("[Consumer] Processing task-%d: %s\n", i, string(taskData))
				time.Sleep(300 * time.Millisecond) // Simulate work

				// Write result
				result := fmt.Sprintf("Completed: %s (at %s)", string(taskData), time.Now().Format("15:04:05"))
				if err := os.WriteFile(resultFile, []byte(result), 0o644); err != nil {
					log.Printf("[Consumer] Failed to write result: %v\n", err)
					continue
				}

				fmt.Printf("[Consumer] Wrote result-%d.txt\n", i)
				processedCount++
			}

			time.Sleep(200 * time.Millisecond)
		}
	}

cleanup:
	// Signal completion
	doneFile := filepath.Join(sharedDir, "consumer-done.flag")
	os.WriteFile(doneFile, []byte("complete"), 0o644)

	// Cleanup
	if err := manager.StopContainerSession(ctx, session.ID()); err != nil {
		log.Printf("[Consumer] Failed to stop session: %v\n", err)
	}
	fmt.Println("[Consumer] Session stopped")

	// Remove container
	if err := dockerClient.ContainerRemove(ctx, session.ContainerID(), container.RemoveOptions{}); err != nil {
		log.Printf("[Consumer] Warning: Failed to remove container: %v\n", err)
	} else {
		fmt.Println("[Consumer] Container removed")
	}
}

// runMonitor watches the shared workspace and reports progress
func runMonitor(ctx context.Context, dockerClient *client.Client, manager *containersession.Manager, sharedDir string) {
	fmt.Println("[Monitor] Starting...")

	// Create session
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "20"})
	if err != nil {
		log.Printf("[Monitor] Failed to create session: %v\n", err)
		return
	}

	// Link to shared directory
	sessionShared := filepath.Join(session.WorkspacePath(), "shared")
	if err := os.Symlink(sharedDir, sessionShared); err != nil {
		log.Printf("[Monitor] Failed to link shared dir: %v\n", err)
		return
	}

	if err := manager.StartContainerSession(ctx, session.ID()); err != nil {
		log.Printf("[Monitor] Failed to start session: %v\n", err)
		return
	}

	fmt.Printf("[Monitor] Session %s started\n", session.ID()[:8])

	// Watch for completion
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println("[Monitor] Timeout")
			goto cleanup
		case <-ticker.C:
			// Check files in shared directory
			entries, err := os.ReadDir(sharedDir)
			if err != nil {
				continue
			}

			taskCount := 0
			resultCount := 0
			producerDone := false
			consumerDone := false

			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "task-") {
					taskCount++
				} else if strings.HasPrefix(name, "result-") {
					resultCount++
				} else if name == "producer-done.flag" {
					producerDone = true
				} else if name == "consumer-done.flag" {
					consumerDone = true
				}
			}

			fmt.Printf("[Monitor] Status: %d tasks, %d results, Producer:%v, Consumer:%v\n",
				taskCount, resultCount, producerDone, consumerDone)

			// Exit when both are done
			if producerDone && consumerDone {
				fmt.Println("[Monitor] Both sessions completed!")

				// Write summary
				summary := fmt.Sprintf("Summary: %d tasks processed successfully\nCompleted at: %s\n",
					resultCount, time.Now().Format("15:04:05"))
				summaryFile := filepath.Join(sharedDir, "summary.txt")
				os.WriteFile(summaryFile, []byte(summary), 0o644)
				fmt.Println("[Monitor] Wrote summary.txt")

				goto cleanup
			}
		}
	}

cleanup:
	// Cleanup
	if err := manager.StopContainerSession(ctx, session.ID()); err != nil {
		log.Printf("[Monitor] Failed to stop session: %v\n", err)
	}
	fmt.Println("[Monitor] Session stopped")

	// Remove container
	if err := dockerClient.ContainerRemove(ctx, session.ContainerID(), container.RemoveOptions{}); err != nil {
		log.Printf("[Monitor] Warning: Failed to remove container: %v\n", err)
	} else {
		fmt.Println("[Monitor] Container removed")
	}
}

// showWorkspaceContents displays files in the shared workspace
func showWorkspaceContents(sharedDir string) {
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		log.Printf("Failed to read shared directory: %v\n", err)
		return
	}

	fmt.Printf("Shared workspace contents (%d files):\n", len(entries))
	for _, entry := range entries {
		filePath := filepath.Join(sharedDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("  %-25s (error reading)\n", entry.Name())
			continue
		}
		contentStr := string(content)
		if len(contentStr) > 60 {
			contentStr = contentStr[:57] + "..."
		}
		fmt.Printf("  %-25s %s\n", entry.Name(), contentStr)
	}
}
