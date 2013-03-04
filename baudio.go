package baudio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	//"os"
	"os/exec"
	"runtime"
	"strconv"
	//"time"
)

const (
	FuncValueTypeFloat    = 0
	FuncValueTypeNotFloat = 1
)

type BChannel struct {
	funcValueType int
	funcs         []func(float64, int) float64
}

func newBChannel(fvt int) *BChannel {
	bc := &BChannel{
		funcValueType: fvt,
		funcs:         make([]func(float64, int) float64, 0),
	}
	return bc
}

func (bc *BChannel) push(fn func(float64, int) float64) {
	bc.funcs = append(bc.funcs, fn)
}

type bOptions struct {
	size int
	rate int
}

func NewBOptions() *bOptions {
	return &bOptions{
		size: 2048,
		rate: 44000,
	}
}

type B struct {
	readable   bool
	size       int
	rate       int
	t          float64
	i          int
	paused     bool
	ended      bool
	destroyed  bool
	channels   []*BChannel
	chEnd      chan bool
	chEndSox   chan bool
	chResume   chan func()
	chNextTick chan bool
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

func New(opts *bOptions, fn func(float64, int) float64) *B {
	b := &B{
		readable:   true,
		size:       2048,
		rate:       44000,
		t:          0,
		i:          0,
		paused:     false,
		ended:      false,
		destroyed:  false,
		chEnd:      make(chan bool),
		chEndSox:   make(chan bool),
		chResume:   make(chan func()),
		chNextTick: make(chan bool),
	}
	b.pipeReader, b.pipeWriter = io.Pipe()
	if opts != nil {
		b.size = opts.size
		b.rate = opts.rate
	}
	if fn != nil {
		b.Push(fn)
	}
	go func() {
		if b.paused {
			b.chResume <- func() {
				go b.loop()
				b.main()
			}
		} else {
			go b.loop()
			b.main()
		}
	}()
	//go b.loop()
	return b
}

func (b *B) main() {
	for {
		// 2013-02-28 koyachi ここで何かしないとループまわらないのなぜ
		// => fmt.PrinfすることでnodeのnextTick的なものがつまれててそのうちPlay()のread待ちまで進めるのでは。
		//L1:
		//fmt.Println("main loop header")
		//fmt.Printf(".")
		//time.Sleep(1 * time.Millisecond)
		runtime.Gosched()
		select {
		case <-b.chEnd:
			fmt.Println("main chEnd")
			b.terminateMain()
			break
		case fn := <-b.chResume:
			//fmt.Println("main chResume")
			fn()
		case <-b.chNextTick:
			//fmt.Println("main chNextTick")
			go b.loop()
			//b.loop()
		default:
			//fmt.Println("main default")
			//go b.loop()
			//goto L1
		}
	}
}

func (b *B) terminateMain() {
	b.pipeWriter.Close()
	b.ended = true
	b.chEndSox <- true
}

func (b *B) end() {
	b.ended = true
}

func (b *B) destroy() {
	b.destroyed = true
	b.chEnd <- true
}

func (b *B) pause() {
	b.paused = true
}

func (b *B) resume() {
	if !b.paused {
		return
	}
	b.paused = false
	b.chResume <- func() {}
}

func (b *B) AddChannel(funcValueType int, fn func(float64, int) float64) {
	bc := newBChannel(funcValueType)
	bc.push(fn)
	b.channels = append(b.channels, bc)
}

func (b *B) PushTo(index int, fn func(float64, int) float64) {
	if len(b.channels) <= index {
		bc := newBChannel(FuncValueTypeFloat)
		b.channels = append(b.channels, bc)
	}
	b.channels[index].funcs = append(b.channels[index].funcs, fn)
}

func (b *B) Push(fn func(float64, int) float64) {
	b.PushTo(len(b.channels), fn)
}

func (b *B) loop() {
	buf := b.tick()
	if b.destroyed {
		// no more events
		//fmt.Println("loop destroyed")
	} else if b.paused {
		//fmt.Println("loop paused")
		b.chResume <- func() {
			b.pipeWriter.Write(buf.Bytes())
			b.chNextTick <- true
		}
	} else {
		//fmt.Println("loop !(destroyed || paused)")
		b.pipeWriter.Write(buf.Bytes())
		if b.ended {
			//fmt.Println("loop ended")
			b.chEnd <- true
		} else {
			//fmt.Println("loop !ended")
			b.chNextTick <- true
		}
	}
}

func (b *B) tick() *bytes.Buffer {
	bufSize := b.size * len(b.channels)
	byteBuffer := make([]byte, 0)
	buf := bytes.NewBuffer(byteBuffer)
	for i := 0; i < bufSize; i += 2 {
		lrIndex := int(i / 2)
		lenCh := len(b.channels)
		ch := b.channels[lrIndex%lenCh]
		t := float64(b.t) + math.Floor(float64(lrIndex))/float64(b.rate)/float64(lenCh)
		counter := b.i + int(math.Floor(float64(lrIndex)/float64(lenCh)))

		value := float64(0)
		n := float64(0)
		for j := 0; j < len(ch.funcs); j++ {
			x := ch.funcs[j](float64(t), counter)
			n += x
		}
		n /= float64(len(ch.funcs))

		if ch.funcValueType == FuncValueTypeFloat {
			value = signed(n)
		} else {
			b_ := math.Pow(2, float64(ch.funcValueType))
			x := math.Mod(math.Floor(n), b_) / b_ * math.Pow(2, 15)
			value = x
		}
		if err := binary.Write(buf, binary.LittleEndian, int16(clamp(value))); err != nil {
			panic(err)
		}
	}
	b.i += b.size / 2
	b.t += float64(b.size) / float64(2) / float64(b.rate)
	return buf
}

func clamp(x float64) float64 {
	return math.Max(math.Min(x, math.Pow(2, 15)-1), -math.Pow(2, 15))
}

func signed(n float64) float64 {
	b := math.Pow(2, 15)
	if n > 0 {
		return math.Min(b-1, math.Floor(b*n-1))
	}
	return math.Max(-b, math.Ceil(b*n-1))
}

func mergeArgs(opts, args map[string]string) []string {
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

func (b *B) runCommand(command string, mergedArgs []string) {
	cmd := exec.Command(command, mergedArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	defer func() {
		fmt.Println("runCommand: before stdin.Close()")
		stdin.Close()
	}()
	var out bytes.Buffer
	cmd.Stdout = &out
	// TODO: option
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if p := cmd.Process; p != nil {
			fmt.Println("runCommand: before p.Kill()")
			p.Kill()
		}
	}()

	readBuf := make([]byte, b.size*len(b.channels))
	for {
		//fmt.Println("play loop header")
		if _, err := b.pipeReader.Read(readBuf); err != nil {
			panic(err)
		}
		if _, err = stdin.Write(readBuf); err != nil {
			// TODO: more better error handling
			if err.Error() == "write |1: broken pipe" {
				fmt.Printf("ERR: stdin.Write(readBuf): err = %v\n", err)
				runtime.Gosched()
				break
			}
			panic(err)
		}
	}
}

func (b *B) Play(opts map[string]string) {
	go b.runCommand("play", mergeArgs(opts, map[string]string{
		"c": strconv.Itoa(len(b.channels)),
		"r": strconv.Itoa(b.rate),
		"t": "s16",
		"-": "DUMMY",
	}))
	<-b.chEndSox
	b.pipeReader.Close()
}

func (b *B) Record(file string, opts map[string]string) {
	go b.runCommand("sox", mergeArgs(opts, map[string]string{
		"c":  strconv.Itoa(len(b.channels)),
		"r":  strconv.Itoa(b.rate),
		"t":  "s16",
		"-":  "DUMMY",
		"-o": file,
	}))
	<-b.chEndSox
	b.pipeReader.Close()
}
