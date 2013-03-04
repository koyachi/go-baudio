package main

import (
	"../../go-baudio"
	"math"
)

func main() {
	b := baudio.New(nil, nil)
	b.Push(func(t float64, i int) float64 {
		return math.Sin(math.Mod(t, 15.0)*150.0*math.Mod(t, 30.0)*math.Floor(math.Sin(t)*5.0)) + float64(int(float64(int(t)<<3)*float64(int(t)&0x7f)))/256
	})
	b.Push(func() func(float64, int) float64 {
		c := 10.0
		return func(t float64, i int) float64 {
			n := 28.0
			c := c * (1.0 + math.Sin(float64(i)/20000.0)/10000.0)
			return math.Sin(t*5000.0) * math.Max(0, math.Sin(t*n+c*math.Sin(t*20.0)))
		}
	}())
	b.Play(nil)
	//b.Record("./ramp.wav", nil)
}
