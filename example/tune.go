package main

import (
	"../../go-baudio"
	"math"
)

func main() {
	b := baudio.New(func(t float64, i int) float64 {
		var flag float64
		if math.Mod(t, 2) > 1 {
			flag = 1
		} else {
			flag = 0
		}
		x := math.Sin(t*400*math.Pi*2) + math.Sin(t*500)*flag
		return x
	})
	b.Play(nil)
	//b.Record("./tune.wav", nil)
}
