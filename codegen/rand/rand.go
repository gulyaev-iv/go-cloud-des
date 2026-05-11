package rand

import (
	"math"
	"math/rand/v2"
)

type Rand struct {
	*rand.Rand
}

func New(src rand.Source) *Rand {
	return &Rand{rand.New(src)}
}

func (r *Rand) Uniform(min, max float64) float64 {
	return min + r.Float64()*(max-min)
}

func (r *Rand) Triangular(min, mod, max float64) float64 {
	u := r.Float64()
	fc := (mod - min) / (max - min)

	if u < fc {
		return min + math.Sqrt(u*(max-min)*(mod-min))
	}
	return max - math.Sqrt((1-u)*(max-min)*(max-mod))
}

func (r *Rand) Exponential(lambda float64) float64 {
	return r.ExpFloat64() / lambda
}

func (r *Rand) Normal(mean, stddev float64) float64 {
	return mean + r.NormFloat64()*stddev
}
