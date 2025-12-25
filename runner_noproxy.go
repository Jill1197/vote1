package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ==================== CONFIG ====================
const (
	TotalWorkers = 5                       // 5 threads
	DelayBetween = 500 * time.Millisecond  // หน่วงระหว่างแต่ละ vote
	VoteExePath  = ".\\dailynews_vote_randomized.exe"
)

// ==================== STATS ====================
var (
	successCount int64
	failCount    int64
	totalCount   int64
)

// ==================== WORKER ====================
func worker(id int, wg *sync.WaitGroup, stopCh <-chan struct{}) {
	defer wg.Done()

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		// รัน .exe โดยไม่ใส่ proxy (ล้าง env)
		cmd := exec.Command(VoteExePath)
		cleanEnv := []string{}
		for _, env := range os.Environ() {
			upper := strings.ToUpper(env)
			if !strings.HasPrefix(upper, "HTTP_PROXY") && !strings.HasPrefix(upper, "HTTPS_PROXY") {
				cleanEnv = append(cleanEnv, env)
			}
		}
		cmd.Env = cleanEnv

		output, err := cmd.CombinedOutput()
		atomic.AddInt64(&totalCount, 1)
		outputStr := string(output)

		if err != nil {
			atomic.AddInt64(&failCount, 1)
			fmt.Printf("[W%d] ✗ ERROR: %v\n", id, err)
		} else if strings.Contains(outputStr, "SUCCESS") || strings.Contains(outputStr, "✓") {
			atomic.AddInt64(&successCount, 1)
			fmt.Printf("[W%d] ✓ SUCCESS\n", id)
		} else if strings.Contains(outputStr, "Cloudflare") {
			atomic.AddInt64(&failCount, 1)
			fmt.Printf("[W%d] ✗ CLOUDFLARE\n", id)
		} else {
			atomic.AddInt64(&failCount, 1)
			fmt.Printf("[W%d] ✗ FAILED\n", id)
		}

		time.Sleep(DelayBetween)
	}
}

// ==================== STATS REPORTER ====================
func statsReporter(stopCh <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	start := time.Now()

	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start)
			total := atomic.LoadInt64(&totalCount)
			success := atomic.LoadInt64(&successCount)
			fail := atomic.LoadInt64(&failCount)
			rate := float64(success) / elapsed.Seconds() * 60
			fmt.Printf("\n[STATS] Total: %d | Success: %d | Failed: %d | Rate: %.1f/min | %v\n\n",
				total, success, fail, rate, elapsed.Round(time.Second))
		case <-stopCh:
			return
		}
	}
}

// ==================== MAIN ====================
func main() {
	fmt.Println("=== Dailynews Vote Runner (No Proxy) ===")
	fmt.Printf("Workers: %d | Delay: %v\n", TotalWorkers, DelayBetween)
	fmt.Println("\nPress Ctrl+C to stop...\n")

	if _, err := os.Stat(VoteExePath); os.IsNotExist(err) {
		fmt.Printf("ERROR: %s not found!\n", VoteExePath)
		fmt.Println("Build: go build -o dailynews_vote_randomized.exe dailynews_vote_randomized.go")
		return
	}

	stopCh := make(chan struct{})
	go statsReporter(stopCh)

	var wg sync.WaitGroup
	fmt.Println("Starting workers...")
	for i := 0; i < TotalWorkers; i++ {
		wg.Add(1)
		go worker(i, &wg, stopCh)
		time.Sleep(100 * time.Millisecond)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n\nStopping...")
	close(stopCh)
	wg.Wait()

	fmt.Println("\n========== FINAL ==========")
	fmt.Printf("Total:   %d\n", totalCount)
	fmt.Printf("Success: %d\n", successCount)
	fmt.Printf("Failed:  %d\n", failCount)
	if totalCount > 0 {
		fmt.Printf("Rate:    %.2f%%\n", float64(successCount)/float64(totalCount)*100)
	}
	fmt.Println("============================")
}
