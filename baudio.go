package baudio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"time"
)

const (
	FuncValueTypeFloat    = 0
	FuncValueTypeNotFloat = 1
)

type BChannel struct {
	funcValueType int
	funcs         []func(float64) float64
}

//func newBChannel(fvt int, fn func(float64) float64) *BChannel {
func newBChannel(fvt int) *BChannel {
	//bc := newBChannel(fvt)
	bc := &BChannel{
		funcValueType: fvt,
		funcs:         make([]func(float64) float64, 0),
	}
	//bc.funcs = append(bc.funcs, fn)
	return bc
}

func (bc *BChannel) push(fn func(float64) float64) {
	bc.funcs = append(bc.funcs, fn)
}

/*
func newBChannel(fvt int) {
	bc := &BChanenl{
		funcValueType: fvt,
		funcs:         make([]func(float64) float64),
	}
	b := NewBaudio(opts)
	b.Push(fn)
	return b
}
*/

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
	chResume   chan func()
	chNextTick chan bool
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

func New( /*opts map[string]string*/ fn func(float64) float64) *B {
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
		chResume:   make(chan func()),
		chNextTick: make(chan bool),
	}
	b.pipeReader, b.pipeWriter = io.Pipe()
	//TODO
	/*
		if val, ok := opts["size"]; ok {
			b.size = val
		}
		if val, ok := opts["rate"]; ok {
			b.rate = val
		}
	*/

	b.Push(0, fn)
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
		time.Sleep(1 * time.Millisecond)
		select {
		case <-b.chEnd:
			//fmt.Println("main chEnd")
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

func (b *B) AddChannel(funcValueType int, fn func(float64) float64) {
	// DEFAULT, TODO: support ohter value type?
	//bc := newBChannel(funcValueType, fn)
	bc := newBChannel(FuncValueTypeFloat)
	bc.push(fn)
	b.channels = append(b.channels, bc)
}

/*
func (b *B) AddChannel(funcValueType int) {
	funcValueType = FuncValueTypeFloat
	bc := newBChannel(funcValueType)
	b.channels = append(b.channels, bc)
}
*/

func (b *B) Push(index int, fn func(float64) float64) {
	if len(b.channels) <= index {
		//b.AddChannel(FuncValueTypeFloat)
		bc := newBChannel(FuncValueTypeFloat)
		b.channels = append(b.channels, bc)
	}
	b.channels[index].funcs = append(b.channels[index].funcs, fn)
}

/*
func (b *B) Push(fn func(float64) float64) {
	b.Push(0, fn)
}
*/

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
		//counter := b.i + int(math.Floor(float64(lrIndex)/float64(lenCh)))

		value := float64(0)
		n := float64(0)
		for j := 0; j < len(ch.funcs); j++ {
			//x := ch.funcs[j](float64(t), counter)
			x := ch.funcs[j](float64(t))
			n += x
		}
		n /= float64(len(ch.funcs))

		if ch.funcValueType == FuncValueTypeFloat {
			value = signed(n)
		} else {
			/*
				b := math.Pow(2, ch.funcValueType)
				x := (math.Floor(n) % b) / b * math.Pow(2, 15)
				value = x
			*/
		}
		err := binary.Write(buf, binary.LittleEndian, int16(clamp(value)))
		if err != nil {
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
	var results []string
	for k, _ := range args {
		switch k {
		case "-":
			results = append(results, k)
		case "-o":
			results = append(results, k, args[k])
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
	fmt.Printf("results = %v\n", results)
	return results
}

func (b *B) runCommand(command string, mergedArgs []string) {
	cmd := exec.Command(command, mergedArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	defer stdin.Close()
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if p := cmd.Process; p != nil {
			p.Kill()
		}
	}()

	readBuf := make([]byte, b.size*len(b.channels))
	for {
		//fmt.Println("play loop header")
		//n, err := b.pipeReader.Read(readBuf)
		_, err := b.pipeReader.Read(readBuf)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("read n = %d\n", n)
		_, err = stdin.Write(readBuf)
		if err != nil {
			panic(err)
		}
	}
}

func (b *B) Play(opts map[string]string) {
	b.runCommand("play", mergeArgs(opts, map[string]string{
		"c": strconv.Itoa(len(b.channels)),
		"r": strconv.Itoa(b.rate),
		"t": "s16",
		"-": "DUMMY",
	}))
}

func (b *B) Record(file string, opts map[string]string) {
	b.runCommand("sox", mergeArgs(opts, map[string]string{
		"c":  strconv.Itoa(len(b.channels)),
		"r":  strconv.Itoa(b.rate),
		"t":  "s16",
		"-":  "DUMMY",
		"-o": file,
	}))
}
