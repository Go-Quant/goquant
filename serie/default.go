package serie

import (
	"fmt"
	"math"
)

// Constants representing the types of operations.
const (
	OpMul = iota
	OpAdd
	OpSub
	OpDiv
	OpCustom
)

type Bar struct {
	Close  float64 `json:"close"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume float64 `json:"volume"`
	Time   float64 `json:"timestamp"`
}

type Serie interface {
	Get() float64
	G(back int) float64 // Get
	B(steps int) Serie  // Back
	Cache(label ...string) Serie

	Add(value any) Serie
	Sub(value any) Serie
	Mul(value any) Serie
	Div(value any) Serie
	Custom(f Func) Serie
	Sign() float64
	NA() bool
	NZ(replacement ...float64) float64

	Set(steps int) *float64
	AddData(bars []float64)
}

type Func func(result float64) float64

// to prevent importing GoQuant as causes import cycle
type IGoQuant interface {
	IncreaseFuncIndex(i int)
	DecreaseFuncIndex(i int)
	BarIndex() int
	BarFuncIndex() int
}

type SerieWrapper struct {
	data       *[]float64
	operations [][2]any // each operation will be a [operationType, factor] pair.
	GoQuant    IGoQuant
	diff       int
}

// NewSerie creates a new instance of Serie.
func NewSerie(g IGoQuant, data []float64, length ...int) Serie {
	l := len(data)
	if l == 0 {
		l = length[0]
	}

	d := make([]float64, l)
	copy(d, data)

	return &SerieWrapper{data: &d, GoQuant: g, diff: g.BarIndex(), operations: [][2]any{}}
}

func (s SerieWrapper) addOperation(opType int, value any) Serie {
	newOperations := append(s.operations, [2]any{opType, value})
	//s.operations = newOperations
	//return ct
	return &SerieWrapper{
		data:       s.data,
		operations: newOperations,
		GoQuant:    s.GoQuant,
		diff:       s.diff,
	}
}

func (s SerieWrapper) Custom(f Func) Serie {
	return s.addOperation(OpCustom, f)
}

func (s SerieWrapper) Mul(value any) Serie {
	return s.addOperation(OpMul, value)
}

func (s SerieWrapper) Add(value any) Serie {
	return s.addOperation(OpAdd, value)
}

func (s SerieWrapper) Sub(value any) Serie {
	return s.addOperation(OpSub, value)
}

func (s SerieWrapper) Div(value any) Serie {
	return s.addOperation(OpDiv, value)
}

func (s SerieWrapper) Cache(label ...string) Serie {
	fmt.Printf("Cache(%s) called without Back(n)\n", label)
	return &s
}

func (s SerieWrapper) Set(steps int) *float64 {
	adjustedIndex := s.GoQuant.BarIndex() - s.GoQuant.BarFuncIndex() - steps - s.diff
	if adjustedIndex < 0 {
		v := math.NaN()
		return &v
	}

	return &(*s.data)[adjustedIndex]
}

func (s SerieWrapper) Sign() float64 {
	v := s.Get()

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

func (s SerieWrapper) NZ(replacement ...float64) float64 {
	rep := 0.0
	if len(replacement) > 0 {
		rep = replacement[0]
	}

	value := s.Get()
	if NA(value) {
		return rep
	}

	return value
}

func (s SerieWrapper) NA() bool {
	return NA(s.Get())
}

func (s SerieWrapper) B(steps int) Serie {
	return NewWrapper(s.GoQuant, func() float64 {
		s.GoQuant.IncreaseFuncIndex(steps)
		res := s.Get()
		s.GoQuant.DecreaseFuncIndex(steps)
		return res
	})
}

// Get returns the result at the given index after applying all operations lazily.
func (s SerieWrapper) Get() float64 {
	adjustedIndex := s.GoQuant.BarIndex() - s.GoQuant.BarFuncIndex() - s.diff

	if adjustedIndex < 0 || adjustedIndex >= len(*s.data) {
		return math.NaN()
	}

	result := (*s.data)[adjustedIndex]

	for _, operation := range s.operations {
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

	return result
}

func (s SerieWrapper) G(i int) float64 {
	if i == 0 {
		return s.Get()
	}
	return s.B(i).Get()
}

func (s SerieWrapper) AddData(bars []float64) {
	data := append(*s.data, bars...)
	*s.data = data
}
