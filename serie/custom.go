package serie

import (
	"fmt"
	"math"
	"runtime"
)

var cache = make(map[string]map[int]float64)

type CustomSerieWrapper struct {
	f          *func() float64
	operations *[][2]any // each operation will be a [operationType, factor] pair
	GoQuant    IGoQuant
	cacheKey   string
}

func NewWrapper(c IGoQuant, f func() float64) Serie {
	return &CustomSerieWrapper{f: &f, GoQuant: c, operations: &[][2]any{}}
}

func (c CustomSerieWrapper) addOperation(opType int, value any) Serie {
	newOperations := append(*c.operations, [2]any{opType, value})
	//c.operations = newOperations
	//return ct
	return &CustomSerieWrapper{
		f:          c.f,
		operations: &newOperations,
		GoQuant:    c.GoQuant,
	}
}

func (c CustomSerieWrapper) Mul(value any) Serie {
	return c.addOperation(OpMul, value)
}

func (c CustomSerieWrapper) Add(value any) Serie {
	return c.addOperation(OpAdd, value)
}

func (c CustomSerieWrapper) Sub(value any) Serie {
	return c.addOperation(OpSub, value)
}

func (c CustomSerieWrapper) Div(value any) Serie {
	return c.addOperation(OpDiv, value)
}

func (c CustomSerieWrapper) Custom(f Func) Serie {
	return c.addOperation(OpCustom, f)
}

func (c CustomSerieWrapper) Cache(label ...string) Serie {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	c.cacheKey = lbl
	return &c
}

func (c CustomSerieWrapper) Set(steps int) *float64 {
	fmt.Println("cannot set on CustomSerieWrapper")
	return nil
}

func (c CustomSerieWrapper) Sign() float64 {
	v := c.Get()

	if math.IsNaN(v) {
		return math.NaN()
	}

	if v == 0 {
		return 0
	}

	if v > 0 {
		return 1
	}

	if v < 0 {
		return -1
	}

	panic("cannot determine the sign")
}

func (c CustomSerieWrapper) NZ(replacement ...float64) float64 {
	rep := 0.0
	if len(replacement) > 0 {
		rep = replacement[0]
	}

	value := c.Get()
	if NA(value) {
		return rep
	}

	return value
}

func (c CustomSerieWrapper) NA() bool {
	return NA(c.Get())
}

func (c CustomSerieWrapper) B(steps int) Serie {
	return NewWrapper(c.GoQuant, func() float64 {
		c.GoQuant.IncreaseFuncIndex(steps)
		res := c.Get()
		c.GoQuant.DecreaseFuncIndex(steps)
		return res
	})
}

// Get returns the result at the given index after applying all operations lazily.
func (c CustomSerieWrapper) Get() float64 {
	index := c.GoQuant.BarIndex() - c.GoQuant.BarFuncIndex() // - c.backOffset

	if c.cacheKey != "" {
		if result, exists := cache[c.cacheKey][index]; exists {
			//fmt.Println("from cache", c.cacheKey)
			return result
		}
	}
	result := (*c.f)()

	/*if NA(result) || index < 0 {
		return math.NaN()
	}*/

	for _, operation := range *c.operations {
		opType := operation[0].(int)
		value := operation[1]

		switch opType {
		case OpMul:
			switch v := value.(type) {
			case float64:
				result *= v
			case Serie:
				result *= v.Get()
			}
		case OpAdd:
			switch v := value.(type) {
			case float64:
				result += v
			case Serie:
				result += v.Get()
			}
		case OpSub:
			switch v := value.(type) {
			case float64:
				result -= v
			case Serie:
				result -= v.Get()
			}
		case OpDiv:
			switch v := value.(type) {
			case float64:
				result /= v
			case Serie:
				result /= v.Get()
			}
		case OpCustom:
			customFunc := value.(Func)
			result = customFunc(result)
		}

	}

	if c.cacheKey != "" {
		if cache[c.cacheKey] == nil {
			cache[c.cacheKey] = make(map[int]float64) // Initialize the inner map
		}
		cache[c.cacheKey][index] = result
	}
	return result
}

func (c CustomSerieWrapper) G(i int) float64 {
	if i == 0 {
		return c.Get()
	}
	return c.B(i).Get()
}

func (c CustomSerieWrapper) AddData(bars []float64) {
	fmt.Println("AddData must not be called on CustomSerieWrapper")
}
