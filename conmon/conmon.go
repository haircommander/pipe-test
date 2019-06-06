package conmon

import (
	"runtime"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const executableName = "pipe"

type Conmon struct {
	parentDir string
}

func New() (*Conmon, error) {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return nil, fmt.Errorf("wasn't able to find filename")
	}
	dir := filepath.Dir(filename)
	return &Conmon{dir}, nil
}

func (c *Conmon) Binary() string {
	return filepath.Join(c.parentDir, executableName)
}

func (c *Conmon) Make() error {
	// if binary exists, leave as be
	if c.checkBinary() {
		return nil
	}
	makeSrc := filepath.Join(c.parentDir, "src")
	cmd := exec.Command("make", "-C", makeSrc)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}

	if !c.checkBinary() {
		return fmt.Errorf("File wasn't created")
	}
	return nil
}

func (c *Conmon) checkBinary() bool {
	binary := filepath.Join(c.parentDir, executableName)
	if _, err := os.Stat(binary); os.IsNotExist(err) {
		return false
	}
	return true
}
