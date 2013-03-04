package main

import (
	"../../go-baudio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"
)

func main() {
	var loopEnd = -1
	if len(os.Args) > 1 {
		var err error
		loopEnd, err = strconv.Atoi(os.Args[len(os.Args)-1])
		if err != nil {
			panic(err)
		}
	}
	rand.Seed(time.Now().UnixNano())
	b := baudio.New(nil, nil)
	b.Push(func() func(float64, int) float64 {
		freqs := []float64{
			0, 0, 1600, 1600,
			0, 0, 2000, 2000,
			0, 1400, 0, 1400,
			0, 1600, 0, 1800,
		}
		mutate := func() []float64 {
			xs := freqs
			ix := int(math.Floor(rand.Float64() * float64(len(xs))))
			xs[ix] = math.Max(0, xs[ix]+((math.Floor(rand.Float64()*2)-1.0)*2.0+1.0)*400)
			return xs
		}

		loop := 0
		return func(t float64, i int) float64 {
			n := int(math.Floor(float64(float64(t)*4.0) / float64(len(freqs))))
			if loop != n {
				loop = n
				if loop == loopEnd {
					b.End()
					return 0.0
				}
				freqs = mutate()
				fmt.Printf("iteration %d  %v\n", loop, freqs)
			}
			f := freqs[int(math.Floor(math.Mod(float64(t)*4.0, float64(len(freqs)))))]
			return math.Sin(t * math.Pi * f)
		}
	}())
	b.Push(func(t float64, i int) float64 {
		f := 800.0 * math.Pow(2.0, math.Floor(math.Mod(float64(t)*4.0, 4.0))/6.0)
		return math.Sin(float64(t)*f*math.Pi) * math.Pow(math.Sin(float64(t)*8.0*math.Pi), 2.0)
	})
	b.Push(func() func(float64, int) float64 {
		n := 0.0
		return func(t float64, i int) float64 {
			if i%10 == 0 {
				n = rand.Float64()
			}
			t_ := float64(t)
			a := 1.0 / 16.0
			b := 1.0 / 256.0
			r1 := math.Mod(t_*2.0, a) < b
			r2 := math.Mod(t_*2.0/32, a) < b
			r3 := math.Mod(t_*2.0/32, a) < b
			if r1 || r2 || r3 {
				return n
			}
			return 0
		}
	}())
	b.Play(nil)
	//b.Record("./evolving.wav", nil)
}
