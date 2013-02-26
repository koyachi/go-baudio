package baudio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	FuncValueTypeFloat    = 0
	FuncValueTypeNotFloat = 1
)

type BChannel struct {
	funcValueType int
	funcs         []func(float32) float32
}

func newBChannel(fvt int, fn func(float32) float32) *BChannel {
	//bc := newBChannel(fvt)
	bc := &BChanenl{
		funcValueType: fvt,
		funcs:         make([]func(float32) float32),
	}
	bc.funcs = append(bc.funcs, fn)
	return bc
}

/*
func newBChannel(fvt int) {
	bc := &BChanenl{
		funcValueType: fvt,
		funcs:         make([]func(float32) float32),
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
	t          int
	i          int
	channels   []BChannel
	chEnd      chan bool
	chResume   chan func()
	chData     chan bytes.Buffer
	chNextTick chan bool
}

func NewBaudio(opts map[string]string) {
	b := &B{
		readable:   true,
		size:       2048,
		rate:       44000,
		t:          0,
		i:          0,
		paused:     bool,
		ended:      bool,
		destroyed:  bool,
		chEnd:      make(chan bool),
		chResume:   make(chan func()),
		chData:     make(chan bytes.Buffer),
		chNextTick: make(chan bool),
	}
	if val, ok := opts["size"]; ok {
		b.size = val
	}
	if val, ok := opts["rate"]; ok {
		b.rate = val
	}

	go func() {
		if b.paused {
			b.chResume <- func() {
				b.main()
			}
		} else {
			b.main()
		}
	}()
	return b
}

func (b *B) main() {
	for {

		select {
		case <-b.chNextTick:
			b.loop()
		case <-b.chEnd:
			break
		case fn := <-b.chResume:
			fn()
		case buf := <-b.chData:
			// send buf to sox-play process. pipe?
			//DUMMY
			fmt.Printf("%0x\n", buf)
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
	b.chResume <- true
}

func (b *B) AddChannel(funcValueType int, fn func(float32) float32) {
	// DEFAULT, TODO: support ohter value type?
	funcValueType = FuncValueTypeFloat
	bc := newBChannel(funcValueType, fn)
	b.channels = append(b.channels, bc)
}

/*
func (b *B) AddChannel(funcValueType int) {
	funcValueType = FuncValueTypeFloat
	bc := newBChannel(funcValueType)
	b.channels = append(b.channels, bc)
}
*/

func (b *B) Push(index int, fn func(float32) float32) {
	if len(b.chanenls) <= index {
		//b.AddChannel(FuncValueTypeFloat)
		bc := newBChannel(funcValueType)
		b.channels = append(b.channels, bc)
	}
	b.channels[index].funcs = append(b.channels[index].funcs, fn)
}

/*
func (b *B) Push(fn func(float32) float32) {
	b.Push(0, fn)
}
*/

func (b *B) loop() {
	buf := b.tick()
	if b.destroyed {
		// no more events
	} else if b.paused {
		b.chResume <- func() {
			b.chData <- buf
			b.chNextTick <- true
		}
	} else {
		b.chData <- buf
		if b.ended {
			b.chEnd <- true
		} else {
			b.chNextTick <- true
		}
	}
}

func (b *B) tick() bytes.Buffer {
	byteBuffer := make([]byte, b.size*len(b.channels))
	buf := bytes.NewBuffer()
	for i := 0; i < buf.Len(); i++ {
		lenCh = len(b.channels)
		ch := b.channels[(i/2)%lenCh]
		t := b.t + math.Floor(i/2)/b.rate/lenCh
		counter := b.i + math.Floor(i/2/lenCh)

		value := 0
		n := 0
		for j := 0; j < len(ch[1]); j++ {
			x := ch[1][k](b, t, counter)
			n += x
		}
		n /= len(ch[1])

		if ch[0] == "float" {
			value = signed(n)
		} else {
			b := math.Pow(2, ch[0])
			x := (math.Floor(n) % b) / b * math.Pow(2, 15)
			value = x
		}
		err := binary.Write(buf, binary.LittleEndian, clamp(value))
		if err != nil {
			panic(err)
		}
	}
	b.i += b.size / 2
	b.t += b.size / 2 / b.rate
	return buf
}

func clamp(x int) {
	return math.Max(math.Min(x, math.Pow(2, 15)-1), -math.Pow(2, 15))
}

func mergeArgs(opts, args []string) {
}

func (b *B) Play(opts []string) {
}

func (b *B) Record(file string, opts []string) {
}

func signed(n int) {
	b := math.Pow(2, 15)
	if n > 0 {
		return math.Min(b-1, math.Floor(b*n-1))
	} else {
		return math.Max(-b, math.Ceil(b*n-1))
	}
}
