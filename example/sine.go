package main

import (
	"../../go-baudio"
	"math"
)

func main() {
	n := 0
	b := baudio.NewBaudio(func(t float32) float32 {
		x := math.Sin(t*262 + math.sin(n))
		n += math.sin(t)
		return x
	})
	b.play()
}
