package core

import (
	"fmt"
	"math"
	"runtime"

	"github.com/Go-Quant/goquant/serie"
)

func (g *GoQuant) barsSince(f func() bool) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		for i := 0; i < len(g.bars); i++ {
			g.IncreaseFuncIndex(i)
			res := f()
			g.DecreaseFuncIndex(i)

			if res {
				return float64(i)
			}
		}

		return math.NaN()
	})
}

func (g *GoQuant) valueWhen(src serie.Serie, f func() bool, occurrence int) serie.Serie {
	times := 0
	return serie.NewWrapper(g, func() float64 {
		for i := 0; i < len(g.bars); i++ {
			g.IncreaseFuncIndex(i)
			res := f()
			g.DecreaseFuncIndex(i)

			if res {
				if times >= occurrence {
					return src.G(i)
				}
				times++
			}
		}

		return math.NaN()
	})
}

func (g *GoQuant) pivotHigh(leftBars int, righBars int, source ...serie.Serie) serie.Serie {
	src := g.high
	if len(source) > 0 {
		src = source[0]
	}

	return serie.NewWrapper(g, func() float64 {
		possiblePivot := src.G(righBars)

		// Check left bars
		for i := 1; i <= leftBars; i++ {
			if src.G(righBars+i) >= possiblePivot {
				return False
			}
		}

		// Check right bars
		for i := 1; i <= righBars; i++ {
			if src.G(righBars-i) >= possiblePivot {
				return False
			}
		}

		return True
	})
}

func (g *GoQuant) pivotLow(leftBars int, righBars int, source ...serie.Serie) serie.Serie {
	src := g.low
	if len(source) > 0 {
		src = source[0]
	}

	return serie.NewWrapper(g, func() float64 {
		possiblePivot := src.G(righBars)

		// Check left bars
		for i := 1; i <= leftBars; i++ {
			if src.G(righBars+i) <= possiblePivot {
				return False
			}
		}

		// Check right bars
		for i := 1; i <= righBars; i++ {
			if src.G(righBars-i) <= possiblePivot {
				return False
			}
		}

		return True
	})
}

func (g *GoQuant) sma(src serie.Serie, length float64) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		sum := 0.0
		for i := 0; i < int(length); i++ {
			sum += src.G(i)
		}

		return sum / length
	})
}
func (g *GoQuant) vwma(src serie.Serie, length float64) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		up := g.sma(src.Mul(g.volume), length)
		down := g.sma(g.volume, length)

		return up.Div(down).Get()
	})
}

func (g *GoQuant) ema(src serie.Serie, length float64, label ...string) serie.Serie {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	return serie.NewWrapper(g, func() float64 {
		sum := g.NewEmptyStorage(lbl)
		alpha := 2 / (length + 1)
		sum1 := sum.G(1)

		var res float64
		if serie.NA(sum1) {
			res = src.Get()
		} else {
			res = alpha*src.Get() + (1-alpha)*serie.NZ(sum1)
		}

		*sum.Set(0) = res
		return res
	}).Cache(BUILT_IN + lbl)
}

func abs(v float64) float64 {
	return math.Abs(v)
}

func (g *GoQuant) atr(length float64, label ...string) serie.Serie {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	return serie.NewWrapper(g, func() float64 {
		var trueRange serie.Serie

		if serie.NA(g.high.G(1)) {
			trueRange = g.high.Sub(g.low)
		} else {
			trueRange = g.Max(
				g.Max(g.high.Sub(g.low), g.high.Sub(g.close.B(1).Custom(abs))),
				g.low.Sub(g.close.B(1)).Custom(abs),
			)
		}

		return g.rma(trueRange, length, lbl).Get()
	}).Cache(BUILT_IN + lbl)
}

func (g *GoQuant) rma(src serie.Serie, length float64, label ...string) serie.Serie {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	return serie.NewWrapper(g, func() float64 {
		alpha := 1 / length

		sum := g.NewEmptyStorage(lbl)
		sum1 := sum.G(1)
		var sum0 float64

		if serie.NA(sum1) {
			sum0 = g.sma(src, length).Get()
		} else {
			sum0 = alpha*src.Get() + (1-alpha)*serie.NZ(sum1)
		}

		*sum.Set(0) = sum0
		return sum0
	}).Cache(BUILT_IN + lbl)
}

func (g *GoQuant) rsi(src serie.Serie, length float64, label ...string) serie.Serie {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	return serie.NewWrapper(g, func() float64 {
		zero := g.NewEmptyStorage(lbl)

		u := g.Max(src.Sub(src.B(1)), zero)
		d := g.Max(src.B(1).Sub(src), zero)

		rs := g.rma(u, length, lbl+"u").Div(g.rma(d, length, lbl+"d"))

		return 100 - 100/(1+rs.Get())
	}).Cache(BUILT_IN + lbl)
}

// returns True or False
// only bullish divergences
func (g *GoQuant) divergence(
	rsiLen int,
	rsiSource serie.Serie,
	lbR int,
	lbL int,
	rangeUpper int,
	rangeLower int,
	label ...string) serie.Serie {

	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	inRange := func(cond func() bool) serie.Serie {
		return g.NewWrapper(func() float64 {
			bars := int(g.barsSince(cond).Get())
			if rangeLower <= bars && bars <= rangeUpper {
				return True
			}

			return False
		})
	}

	return g.NewWrapper(func() float64 {
		osc := g.rsi(rsiSource, float64(rsiLen), lbl)

		// -----------
		pl :=
			g.pivotLow(lbL, lbR, osc)

		// if the current rsi bar isn't a pivot low
		if pl.Get() == False {
			return False
		}
		// -----------

		// -----------
		// if the current rsi bar isn't higher low
		oscHL :=
			inRange(func() bool { return pl.G(1) == True }).Get() == True &&
				osc.G(lbR) > g.valueWhen(osc.B(lbR), func() bool { return pl.Get() == True }, 1).Get()

		if !oscHL {
			return False
		}
		// -----------

		// -----------
		// if the current bar isn't lower low
		priceLL :=
			g.low.B(lbR).Get() < g.valueWhen(g.low.B(lbR), func() bool { return pl.Get() == True }, 1).Get()

		if !priceLL {
			return False
		}
		// -----------

		return True
	}).Cache(BUILT_IN + lbl)
}
