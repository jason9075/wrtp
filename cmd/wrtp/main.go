package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

const (
	lockFile      = "/tmp/wrtp.lock"
	tempInputFile = "/tmp/wrtp.yml"
	posFile       = "/tmp/wrtp.pos"
)

func saveCursorPos() {
	out, err := exec.Command("hyprctl", "cursorpos").Output()
	if err != nil {
		// Silently fail if not on Hyprland or hyprctl fails
		return
	}
	_ = os.WriteFile(posFile, out, 0644)
}

func restoreCursorPos() {
	data, err := os.ReadFile(posFile)
	if err != nil {
		return
	}

	// hyprctl cursorpos returns "x, y"
	var x, y int
	n, err := fmt.Sscanf(string(data), "%d, %d", &x, &y)
	if err != nil || n != 2 {
		return
	}

	fmt.Printf("Restoring cursor to %d, %d\n", x, y)
	_ = exec.Command("hyprctl", "dispatch", "movecursor", strconv.Itoa(x), strconv.Itoa(y)).Run()
}

func main() {
	if os.Getuid() == 0 {
		fmt.Fprintf(os.Stderr, "Warning: Running as root is not recommended.\n")
		fmt.Fprintf(os.Stderr, "This program handles 'sudo' internally for input recording.\n")
		fmt.Fprintf(os.Stderr, "Running as root may cause the UI overlay to fail due to X11/Wayland display permissions.\n\n")
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  wrtp is a daemon-free state toggle utility for recording mouse and keyboard actions on Wayland.\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  Running it without flags will toggle (start/stop) the recording.\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  NOTE: Most operations require 'sudo' to access /dev/input devices.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "  -h, -help\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    \tShow this help message\n")
	}

	testDuration := flag.Int("test", 0, "Record for N seconds and play once")
	playMode := flag.Bool("play", false, "Replay the latest recording")
	flag.Parse()

	if *testDuration > 0 {
		runTestMode(*testDuration)
		return
	}

	if *playMode {
		checkLibinputQuirks()
		restoreCursorPos()
		Play()
		return
	}

	if exists(lockFile) {
		stopExisting()
		return
	}

	startRecording()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func stopExisting() {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		handleError(err, "reading lock file")
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid PID in lock file: %v\n", err)
		os.Exit(1)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		handleError(err, fmt.Sprintf("finding process %d", pid))
	}

	fmt.Printf("Sending SIGINT to process %d...\n", pid)
	err = process.Signal(os.Interrupt)
	if err != nil {
		handleError(err, "signaling process")
	}

	// Wait a bit for the other process to clean up
	for i := 0; i < 15; i++ {
		if !exists(lockFile) {
			fmt.Println("Stopped successfully.")
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("Process signaled, but lock file still exists. Forcing cleanup.")
	err = os.Remove(lockFile)
	if err != nil {
		handleError(err, "removing lock file")
	}
}

func startRecording() {
	saveCursorPos()
	pid := os.Getpid()
	err := os.WriteFile(lockFile, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		handleError(err, "creating lock file")
	}

	fmt.Printf("Started recording (PID: %d). Press Ctrl+C or run again to stop.\n", pid)

	// Setup overlay UI on main thread
	overlayApp := app.New()
	w := createOverlayWindow(overlayApp)

	// Setup signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		// Run recording logic in background
		Record(done, 0)
		fyne.Do(func() {
			w.Close()
		})
	}()

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal: %v. Cleaning up...\n", sig)
		cleanup()
		fyne.Do(func() {
			w.Close()
		})
		done <- true
	}()

	w.ShowAndRun()
	fmt.Println("\nRecording finished.")
}

func runTestMode(seconds int) {
	fmt.Printf("Test Mode: Recording for %d seconds...\n", seconds)
	saveCursorPos()
	
	// Setup overlay UI on main thread
	overlayApp := app.New()
	w := createOverlayWindow(overlayApp)

	// Setup signal handling to allow interrupting test mode
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	done := make(chan bool, 1)
	interrupted := false

	dur := time.Duration(seconds) * time.Second

	// Automatically signal done after duration OR if Ctrl+C pressed
	go func() {
		select {
		case <-time.After(dur):
			done <- true
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal: %v. Stopping test mode...\n", sig)
			interrupted = true
			done <- true
		}
	}()

	go func() {
		// Run test logic in background
		Record(done, dur)
		fyne.Do(func() {
			w.Close()
		})
	}()

	w.ShowAndRun()

	if interrupted {
		return
	}

	fmt.Println("\nRecording finished. Playing back once...")
	checkLibinputQuirks()
	restoreCursorPos()
	Play()
	fmt.Println("Playback finished.")
}

func createOverlayWindow(a fyne.App) fyne.Window {
	w := a.NewWindow("wrtp-overlay")
	w.SetFixedSize(true)
	w.Resize(fyne.NewSize(120, 40))
	w.SetPadded(false)

	// Create red dot
	dot := canvas.NewCircle(color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	dot.Resize(fyne.NewSize(16, 16))

	// Create REC text
	text := canvas.NewText("REC", color.White)
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = 20

	content := container.New(layout.NewHBoxLayout(),
		layout.NewSpacer(),
		container.New(layout.NewCenterLayout(), dot),
		text,
		layout.NewSpacer(),
	)

	w.SetContent(content)
	return w
}

func checkLibinputQuirks() {
	const quirksPath = "/etc/libinput/local-overrides.quirks"
	if exists(quirksPath) {
		fmt.Fprintf(os.Stderr, "\nWarning: %s exists.\n", quirksPath)
		fmt.Fprintf(os.Stderr, "libinput replay will fail on some systems if this file exists.\n")
		fmt.Fprintf(os.Stderr, "Suggested fix: sudo mv %s %s.bak\n\n", quirksPath, quirksPath)
	}
}

func cleanup() {
	if exists(lockFile) {
		err := os.Remove(lockFile)
		if err != nil {
			handleError(err, "cleaning up lock file")
		}
	}
}

// Record starts libinput record and waits for completion or done signal.
func Record(done chan bool, limit time.Duration) {
	// Remove existing recording if it exists
	if exists(tempInputFile) {
		os.Remove(tempInputFile)
	}

	// Using sudo libinput record --all --show-keycodes -o <file>
	cmd := exec.Command("sudo", "libinput", "record", "--all", "--show-keycodes", "-o", tempInputFile)
	
	// Capture stderr to debug failures
	stderr, err := cmd.StderrPipe()
	if err != nil {
		handleError(err, "getting stderr pipe for libinput record")
	}

	if err := cmd.Start(); err != nil {
		handleError(err, "starting libinput record")
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	msgChan := make(chan error, 1)
	go func() {
		msgChan <- cmd.Wait()
	}()

	for {
		select {
		case err := <-msgChan:
			if err != nil {
				// If it's a signal error (SIGINT), that's expected
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signal() == syscall.SIGINT {
						return
					}
					// Read remaining stderr for context
					buf := make([]byte, 1024)
					n, _ := stderr.Read(buf)
					if n > 0 {
						fmt.Fprintf(os.Stderr, "libinput record error output:\n%s\n", string(buf[:n]))
					}
				}
				handleError(err, "libinput record finished with error")
			}
			return
		case <-done:
			// Send SIGINT to libinput-record to stop it gracefully
			_ = cmd.Process.Signal(os.Interrupt)
			// wait for the msgChan to return from the cmd.Wait() above
			<-msgChan
			return
		case t := <-ticker.C:
			elapsed := t.Sub(startTime).Round(time.Second)
			if limit > 0 {
				fmt.Printf("\rRecording... %v / %v", elapsed, limit)
			} else {
				fmt.Printf("\rRecording... %v", elapsed)
			}
		}
	}
}

// Play replays the recorded input using libinput replay.
func Play() {
	if !exists(tempInputFile) {
		fmt.Fprintf(os.Stderr, "Error: No recording found at %s\n", tempInputFile)
		return
	}

	fmt.Printf("Replaying %s...\n", tempInputFile)
	// libinput replay waits for a newline to start replaying by default.
	// We use --replay-after 0 to start immediately without a prompt.
	// We use --once to ensure it only replays once and then exits.
	cmd := exec.Command("sudo", "libinput", "replay", "--replay-after", "0", "--once", tempInputFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		handleError(err, "starting libinput replay")
	}

	// Setup signal handling to allow interrupting playback
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	msgChan := make(chan error, 1)
	go func() {
		msgChan <- cmd.Wait()
	}()

	select {
	case err := <-msgChan:
		if err != nil {
			// If it's a signal error (SIGINT), that's expected
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signal() == syscall.SIGINT {
					return
				}
			}
			handleError(err, "replaying input with libinput replay")
		}
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v. Stopping replay...\n", sig)
		// Send SIGINT to libinput replay
		_ = cmd.Process.Signal(os.Interrupt)
		<-msgChan // Wait for the subprocess to exit
	}
}

func handleError(err error, context string) {
	if err == nil {
		return
	}

	if errors.Is(err, os.ErrPermission) {
		fmt.Fprintf(os.Stderr, "Error: Permission denied while %s. Please try running with 'sudo'.\n", context)
	} else {
		fmt.Fprintf(os.Stderr, "Error %s: %v\n", context, err)
	}
	os.Exit(1)
}
