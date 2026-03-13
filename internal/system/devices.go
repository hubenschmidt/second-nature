package system

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"second-nature/internal/model"
)

func ListMonitorSources() ([]model.PulseSource, error) {
	out, err := exec.Command("pactl", "list", "short", "sources").Output()
	if err != nil {
		return nil, fmt.Errorf("pactl: %w", err)
	}

	var monitors []model.PulseSource
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasSuffix(fields[1], ".monitor") {
			monitors = append(monitors, model.PulseSource{ID: fields[0], Name: fields[1]})
		}
	}
	return monitors, nil
}

func SelectAudioMode(scanner *bufio.Scanner) (model.CaptureMode, string, error) {
	fmt.Println("\nAudio capture mode:")
	fmt.Println("  1: Mic only")
	fmt.Println("  2: System audio (monitor source)")
	fmt.Println("  3: Mic + System")
	fmt.Print("Choice [1]: ")
	scanner.Scan()

	input := strings.TrimSpace(scanner.Text())
	if input == "" || input == "1" {
		return model.CaptureModeMic, "", nil
	}

	if input != "2" && input != "3" {
		return 0, "", fmt.Errorf("invalid audio mode: %s", input)
	}

	mode := model.CaptureModeSystem
	if input == "3" {
		mode = model.CaptureModeBoth
	}

	source, err := selectMonitorDevice(scanner)
	if err != nil {
		return 0, "", err
	}
	return mode, source, nil
}

func selectMonitorDevice(scanner *bufio.Scanner) (string, error) {
	monitors, err := ListMonitorSources()
	if err != nil {
		return "", err
	}
	if len(monitors) == 0 {
		return "", fmt.Errorf("no monitor sources found — is PulseAudio/PipeWire running?")
	}
	if len(monitors) == 1 {
		fmt.Printf("  Using monitor: %s\n", monitors[0].Name)
		return monitors[0].Name, nil
	}

	fmt.Println("\nSelect monitor source:")
	for i, m := range monitors {
		fmt.Printf("  %d: %s\n", i+1, m.Name)
	}
	fmt.Print("Choice [1]: ")
	scanner.Scan()

	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return monitors[0].Name, nil
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(monitors) {
		return "", fmt.Errorf("invalid monitor selection: %s", input)
	}
	return monitors[choice-1].Name, nil
}
