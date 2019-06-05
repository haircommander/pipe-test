package pipe

import (
	"fmt"
	"encoding/json"
	"os"
	"os/exec"
	"bufio"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)


func must(t *testing.T, err error) {
	if err != nil {
		t.Error(err)
	}
}

func fails(t *testing.T, err error) {
	if err == nil {
		t.Error(err)
	}
}

func readConmonPipeData(pipe *os.File) (int, error) {
	ContainerCreateTimeout := 240 * time.Second
	// syncInfo is used to return data from monitor process to daemon
	type syncInfo struct {
		Data    int    `json:"data"`
		Message string `json:"message,omitempty"`
	}
	// Wait to get container pid from conmon
	type syncStruct struct {
		si  *syncInfo
		err error
	}
	ch := make(chan syncStruct)
	go func() {
		var si *syncInfo
		rdr := bufio.NewReader(pipe)
		b, err := rdr.ReadBytes('\n')
		if err != nil && len(b) == 0 {
			ch <- syncStruct{err: fmt.Errorf("got: %s when reading bytes: %v", string(b), err)}
		}
		if err := json.Unmarshal(b, &si); err != nil {
			ch <- syncStruct{err: fmt.Errorf("got: %s when unmarshalling: %v", string(b), err)}
			return
		}
		ch <- syncStruct{si: si}
	}()

	data := -1
	select {
	case ss := <-ch:
		if ss.err != nil {
			return -1, fmt.Errorf("error reading container (probably exited) json message: %v", ss.err)
		}
		if ss.si.Data < 0 {
			if ss.si.Message != "" {
				return ss.si.Data, fmt.Errorf("container create failed: %s", ss.si.Message)
			}
			return ss.si.Data, fmt.Errorf("container create failed")
		}
		data = ss.si.Data
	case <-time.After(ContainerCreateTimeout):
		return -1, fmt.Errorf("container creation timeout")
	}
	return data, nil
}

func TestPipes(t *testing.T) {
	parentSyncPipe, childSyncPipe, err := newPipe()
	must(t, err)

	defer parentSyncPipe.Close()
	childEndPipe, parentEndPipe, err := newPipe()
	must(t, err)
	defer parentEndPipe.Close()


    ex, err := os.Executable()
	must(t, err)

	dir := filepath.Dir(ex)
	must(t, err)
	cmd := exec.Command(filepath.Join(dir, "pipe"))
	cmd.ExtraFiles = append(cmd.ExtraFiles, childSyncPipe, childEndPipe)
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_ENDPIPE=%d", 3))

	err = cmd.Start()

	// We don't need childPipe on the parent side
	childSyncPipe.Close()
	must(t, err)

	_, err = readConmonPipeData(parentSyncPipe)
	must(t, err)

	_, err = readConmonPipeData(parentSyncPipe)
	must(t, err)

}

// newPipe creates a unix socket pair for communication
func newPipe() (parent, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}
