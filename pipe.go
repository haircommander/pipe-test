package main

import (
	"fmt"
	"encoding/json"
	"os"
	"os/exec"
	"bufio"
	"time"

	"golang.org/x/sys/unix"
	"github.com/haircommander/pipe-test/conmon"
)

func main() { os.Exit(mainReturnWithCode()) }

func mainReturnWithCode() int {
	parentSyncPipe, childSyncPipe, err := newPipe()
	if err != nil {
		fmt.Println(err.Error())
		return 1
	}

	defer parentSyncPipe.Close()
	childEndPipe, parentEndPipe, err := newPipe()
	if err != nil {
		fmt.Println(err.Error())
		return 1
	}
	defer parentEndPipe.Close()

	c, err := conmon.New()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	c.Make()


	cmd := exec.Command(c.Binary())
	cmd.ExtraFiles = append(cmd.ExtraFiles, childSyncPipe, childEndPipe)
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_ENDPIPE=%d", 4))

	//cmd.Stdout = os.Stderr
	//cmd.Stderr = os.Stderr

	err = cmd.Start()

	// We don't need childPipe on the parent side
	childSyncPipe.Close()
	childEndPipe.Close()
	if err != nil {
		fmt.Println(err.Error())
		return 1
	}

	//err = cmd.Wait()

	//if exitErr, ok := err.(*exec.ExitError); ok {
	//	if status, ok := exitErr.Sys().(unix.WaitStatus); ok {
	//		fmt.Println("exited with %d", status)
	//		if err != nil {
	//			fmt.Println(err.Error())
	//			return 1
	//		}
	//	}
	//}
	//if err != nil {
	//	fmt.Println(err.Error())
	//	return 1
	//}

	_, err = readConmonPipeData(parentSyncPipe)
	if err != nil {
		fmt.Println(err.Error())
		return 1
	}
	fmt.Println("got one!")

	_, err = readConmonPipeData(parentSyncPipe)
	if err != nil {
		fmt.Println(err.Error())
		return 1
	}

	fmt.Println("got two!")
	someData := []byte{0}
	_, err = parentEndPipe.Write(someData)
	if err != nil {
		cmd.Wait()
		fmt.Println(err)
		return 1
	}
	if err = cmd.Wait(); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
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
		if err != nil {
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

// newPipe creates a unix socket pair for communication
func newPipe() (parent, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
	//return os.Pipe()
}
