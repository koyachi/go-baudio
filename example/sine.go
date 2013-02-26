package main

import (
	"../../go-baudio"
	"math"
)

func main() {
	n := float64(0)
	b := baudio.NewBaudio(func(t float64) float64 {
		x := math.Sin(t*262 + math.Sin(n))
		n += math.Sin(t)
		return x
	})
	b.Play()
}
