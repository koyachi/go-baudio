package baudio

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime"
)

type Sox struct {
	fn         func() []byte
	pipeWriter io.WriteCloser
}

func NewSox(command string, mergedArgs []string, fn func() []byte) (*Sox, error) {
	sox := &Sox{
		fn: fn,
	}

	cmd := exec.Command(command, mergedArgs...)
	pipeWriter, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer func() {
		fmt.Println("runCommand: before pipeWriter.Close()")
		pipeWriter.Close()
	}()
	sox.pipeWriter = pipeWriter
	var out bytes.Buffer
	cmd.Stdout = &out
	// TODO: option
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		if p := cmd.Process; p != nil {
			fmt.Println("runCommand: before p.Kill()")
			p.Kill()
		}
	}()

	sox.main()
	return sox, nil
}

func (sox *Sox) main() {
	for {
		readBuf := sox.fn()
		if _, err := sox.pipeWriter.Write(readBuf); err != nil {
			// TODO: more better error handling
			if err.Error() == "write |1: broken pipe" {
				fmt.Printf("ERR: sox.pipeWriter.Write(readBuf): err = %v\n", err)
				runtime.Gosched()
				break
			}
			panic(err)
		}
	}
}

func SoxPlay(opts []string, fn func() []byte) (*Sox, error) {
	return NewSox("play", opts, fn)
}

func SoxRecord(file string, opts []string, fn func() []byte) (*Sox, error) {
	return NewSox("sox", opts, fn)
}

func mergeArgs(opts, args RuntimeOption) []string {
	for k, _ := range opts {
		args[k] = opts[k]
	}
	var resultsLast []string
	var results []string
	for k, _ := range args {
		switch k {
		case "-":
			resultsLast = append(resultsLast, k)
		case "-o":
			resultsLast = append(resultsLast, k, args[k])
		default:
			var dash string
			if len(k) == 1 {
				dash = "-"
			} else {
				dash = "--"
			}
			results = append(results, dash+k, args[k])
		}
	}
	results = append(results, resultsLast...)
	fmt.Printf("results = %v\n", results)
	return results
}
