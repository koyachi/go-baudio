package main

import (
	"../../go-baudio"
	"math"
)

func main() {
	n := float64(0)
	b := baudio.New(func(t float64) float64 {
		x := math.Sin(t*262 + math.Sin(n))
		n += math.Sin(t)
		return x
	})
	b.Play(nil)
	//b.Record("./sine.wav", nil)
}
