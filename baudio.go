package baudio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
	//"time"
)

const (
	FuncValueTypeFloat    = 0
	FuncValueTypeNotFloat = 1
)

type GeneratorFunc func(t float64, i int) float64
type RuntimeOption map[string]string

type AudioChannel struct {
	funcValueType int
	funcs         []GeneratorFunc
}

func newAudioChannel(fvt int) *AudioChannel {
	ac := &AudioChannel{
		funcValueType: fvt,
		funcs:         make([]GeneratorFunc, 0),
	}
	return ac
}

func (ac *AudioChannel) push(fn GeneratorFunc) {
	ac.funcs = append(ac.funcs, fn)
}

type AudioBufferOption struct {
	Size int
	Rate int
}

func NewAudioBufferOption() *AudioBufferOption {
	return &AudioBufferOption{
		Size: 2048,
		Rate: 44000,
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
	channels   []*AudioChannel
	chEnd      chan bool
	chEndSox   chan bool
	chResume   chan func()
	chNextTick chan bool
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	sox        *Sox
}

func New(opts *AudioBufferOption, fn GeneratorFunc) *B {
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
		b.size = opts.Size
		b.rate = opts.Rate
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

// TODO: To Go Style (end,destroy,pause,resume are node.js's Stream interface.)
func (b *B) End() {
	b.ended = true
}

func (b *B) Destroy() {
	b.destroyed = true
	b.chEnd <- true
}

func (b *B) Pause() {
	b.paused = true
}

func (b *B) Resume() {
	if !b.paused {
		return
	}
	b.paused = false
	b.chResume <- func() {}
}

func (b *B) AddChannel(funcValueType int, fn GeneratorFunc) {
	ac := newAudioChannel(funcValueType)
	ac.push(fn)
	b.channels = append(b.channels, ac)
}

func (b *B) Push(fn GeneratorFunc) {
	index := len(b.channels)
	if len(b.channels) <= index {
		ac := newAudioChannel(FuncValueTypeFloat)
		b.channels = append(b.channels, ac)
	}
	b.channels[index].funcs = append(b.channels[index].funcs, fn)
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

func (b *B) Play(opts RuntimeOption) {
	go SoxPlay(mergeArgs(opts, RuntimeOption{
		"c": strconv.Itoa(len(b.channels)),
		"r": strconv.Itoa(b.rate),
		"t": "s16",
		"-": "DUMMY",
	}), b.waveReceiver())
	<-b.chEndSox
	b.pipeReader.Close()
}

func (b *B) Record(file string, opts RuntimeOption) {
	go SoxRecord(file, mergeArgs(opts, RuntimeOption{
		"c": strconv.Itoa(len(b.channels)),
		"r": strconv.Itoa(b.rate),
		"t": "s16",
		"-": "DUMMY",
	}), b.waveReceiver())
	<-b.chEndSox
	b.pipeReader.Close()
}

func (b *B) waveReceiver() func() []byte {
	readBuf := make([]byte, b.size*len(b.channels))
	return func() []byte {
		if _, err := b.pipeReader.Read(readBuf); err != nil {
			panic(err)
		}
		return readBuf
	}
}
