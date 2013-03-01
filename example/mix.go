package main

import (
	"../../go-baudio"
	"math"
)

func main() {
	//n := float64(0)
	b := baudio.New(nil)
	b.AddChannel(8, func(t float64, i int) float64 {
		return float64((i & 0x71) * int(math.Floor(float64(i/1000))))
	})
	b.Play(nil)
	//b.Record("./sine.wav", nil)
}
