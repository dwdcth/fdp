package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type Config struct {
	Aria2cPath string
	Split      int
	Conn       int
}

func Download(cfg Config, task Task) error {
	if err := os.MkdirAll(filepath.Dir(task.Path), 0o755); err != nil {
		return err
	}
	args := []string{
		"--continue=true",
		"--allow-overwrite=true",
		"--auto-file-renaming=false",
		"--split=" + strconv.Itoa(cfg.Split),
		"--max-connection-per-server=" + strconv.Itoa(cfg.Conn),
		"--dir=" + filepath.Dir(task.Path),
		"--out=" + filepath.Base(task.Path),
	}
	for _, header := range task.Headers {
		args = append(args, "--header="+header)
	}
	args = append(args, task.URL)
	cmd := exec.Command(cfg.Aria2cPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aria2c download %s failed: %w", task.Digest, err)
	}
	return nil
}
