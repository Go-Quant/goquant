package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"runtime"

	assets "github.com/Go-Quant/goquant"
	"github.com/Go-Quant/goquant/serie"
)

type GoQuant struct {
	loopIndex     int
	loopFuncIndex int
	bars          []serie.Bar
	taStorage     map[string]serie.Serie
	plotStorage   map[string]PlotData
	lineStorage   []LineData

	open   serie.Serie
	high   serie.Serie
	close  serie.Serie
	low    serie.Serie
	volume serie.Serie
	time   serie.Serie
}

type TA struct {
	EMA        func(src serie.Serie, length float64, label ...string) serie.Serie
	ATR        func(length float64, label ...string) serie.Serie
	SMA        func(src serie.Serie, length float64) serie.Serie
	VWMA       func(src serie.Serie, length float64) serie.Serie
	RMA        func(src serie.Serie, length float64, label ...string) serie.Serie
	RSI        func(src serie.Serie, length float64, label ...string) serie.Serie
	Divergence func(rsiLen int, rsiSource serie.Serie, lbR, lbL, rangeUpper, rangeLower int, label ...string) serie.Serie

	Cross      func(src1, src2 serie.Serie) serie.Serie
	CrossOver  func(src1, src2 serie.Serie) serie.Serie
	CrossUnder func(src1, src2 serie.Serie) serie.Serie
	BarsSince  func(condFunc func() bool) serie.Serie
	ValueWhen  func(src serie.Serie, condFunc func() bool, occurrence int) serie.Serie
	PivotHigh  func(leftBars, righBars int, source ...serie.Serie) serie.Serie
	PivotLow   func(leftBars, righBars int, source ...serie.Serie) serie.Serie
}

const BUILT_IN = "_"
const True = float64(1)
const False = float64(0)

type LineType string

const (
	HorizontalStraightLine LineType = "horizontalStraightLine"
	VerticalStraightLine   LineType = "verticalStraightLine"
	Segment                LineType = "segment"
)

type PlotPoint struct {
	Value *float64 `json:"value"`
	Index int      `json:"index"`
	Time  int      `json:"timestamp"`
}
type PlotData struct {
	Data   []PlotPoint `json:"data"`
	Config PlotConfig  `json:"config"`
}

type LineData struct {
	Type   LineType   `json:"type"`
	Config LineConfig `json:"config"`
	Points []Point    `json:"points"`
}

type LineConfig struct {
	Color    string  `json:"color,omitempty"`
	Width    float64 `json:"width,omitempty"`
	Dashed   float64 `json:"dashed,omitempty"`
	Location string  `json:"location,omitempty"`
	Shift    int     `json:"shift,omitempty"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// // //

func New() *GoQuant {
	return &GoQuant{taStorage: make(map[string]serie.Serie), plotStorage: make(map[string]PlotData)}
}

func (g *GoQuant) AddBars(bars []serie.Bar) {
	g.bars = append(g.bars, bars...)

	// storages
	for _, record := range g.taStorage {
		record.AddData(make([]float64, len(bars)))
	}

	// first time calling AddBars
	if g.open == nil {
		g.open = serie.NewSerie(g, serie.ExtractField(bars, "Open"))
		g.close = serie.NewSerie(g, serie.ExtractField(bars, "Close"))
		g.high = serie.NewSerie(g, serie.ExtractField(bars, "High"))
		g.low = serie.NewSerie(g, serie.ExtractField(bars, "Low"))
		g.volume = serie.NewSerie(g, serie.ExtractField(bars, "Volume"))
		g.time = serie.NewSerie(g, serie.ExtractField(bars, "Time"))
		return
	}

	g.open.AddData(serie.ExtractField(bars, "Open"))
	g.close.AddData(serie.ExtractField(bars, "Close"))
	g.high.AddData(serie.ExtractField(bars, "High"))
	g.low.AddData(serie.ExtractField(bars, "Low"))
	g.time.AddData(serie.ExtractField(bars, "Time"))
	g.volume.AddData(serie.ExtractField(bars, "Volume"))
}

func (g *GoQuant) NewStorage(label string, data *[]float64) serie.Serie {
	if serie, exists := g.taStorage[label]; exists {
		return serie
	}

	s := serie.NewSerie(g, *data, len(g.bars))
	g.taStorage[label] = s
	return s
}

func (g *GoQuant) NewEmptyStorage(label string) serie.Serie {
	return g.NewStorage(label, &[]float64{})
}

func (g *GoQuant) NewWrapper(f func() float64) serie.Serie {
	return serie.NewWrapper(g, f)
}

func (g *GoQuant) BarIndex() int {
	return g.loopIndex
}
func (g *GoQuant) BarFuncIndex() int {
	return g.loopFuncIndex
}
func (g *GoQuant) IncreaseFuncIndex(steps int) {
	g.loopFuncIndex += steps
}
func (g *GoQuant) DecreaseFuncIndex(steps int) {
	g.loopFuncIndex -= steps
}
func (g *GoQuant) IsFirstBar() bool {
	return g.loopIndex == 0
}
func (g *GoQuant) IsLastBar() bool {
	return g.loopIndex == len(g.bars)
}

type PlotConfig struct {
	Color     string  `json:"color,omitempty"`
	Width     float64 `json:"width,omitempty"`
	Dashed    float64 `json:"dashed,omitempty"`
	Smooth    int     `json:"smooth,omitempty"`
	Precision int     `json:"precision,omitempty"`
	Location  string  `json:"location,omitempty"`
	Shift     int     `json:"shift,omitempty"`
}

type PlotF func(value float64, config *PlotConfig, label ...string)
type LineF func(p1, p2 Point, config *LineConfig)
type HLineF func(value float64, config *LineConfig)
type VLineF func(config *LineConfig)

func (g *GoQuant) Max(src1, src2 serie.Serie) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		return math.Max(src1.Get(), src2.Get())
	})
}

func (g *GoQuant) Min(src1, src2 serie.Serie) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		return math.Min(src1.Get(), src2.Get())
	})
}

func (g *GoQuant) Change(src serie.Serie, length ...int) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		step := 1
		if len(length) > 0 {
			step = length[0]
			if step == 0 {
				return src.Get()
			}
		}

		return src.Sub(src.B(step)).Get()
	})
}

func (g *GoQuant) crossOver(src1, src2 serie.Serie) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		if src1.Get() > src2.Get() && src1.B(1).Get() <= src2.B(1).Get() {
			return True
		}
		return False
	})
}
func (g *GoQuant) crossUnder(src1, src2 serie.Serie) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		if src1.Get() < src2.Get() && src1.B(1).Get() >= src2.B(1).Get() {
			return True
		}
		return False
	})
}
func (g *GoQuant) cross(src1, src2 serie.Serie) serie.Serie {
	return serie.NewWrapper(g, func() float64 {
		s1 := src1.Get()
		s2 := src2.Get()
		s1_1 := src1.G(1)
		s2_1 := src2.G(1)

		if s1 > s2 && s1_1 <= s2_1 {
			return True
		}

		if s1 < s2 && s1_1 >= s2_1 {
			return True
		}

		return False
	})
}

func (g *GoQuant) vline(config *LineConfig) {
	if config == nil {
		config = &LineConfig{}
	}

	if config.Location == "" {
		config.Location = "candle_pane"
	}

	index := g.loopIndex + config.Shift
	if index < 0 || index >= len(g.bars) {
		return
	}

	bar := g.bars[index]
	mid := (bar.Open + bar.Close) / 2
	x := bar.Time

	g.lineStorage = append(g.lineStorage, LineData{Type: VerticalStraightLine, Config: *config, Points: []Point{{X: x, Y: mid}}})
}

func (g *GoQuant) hline(value float64, config *LineConfig) {
	if config == nil {
		config = &LineConfig{}
	}

	if config.Location == "" {
		config.Location = "candle_pane"
	}

	index := g.loopIndex + config.Shift
	if index < 0 || index >= len(g.bars) {
		return
	}

	x := g.bars[index].Time

	g.lineStorage = append(g.lineStorage, LineData{Type: HorizontalStraightLine, Config: *config, Points: []Point{{X: x, Y: value}}})
}

func (g *GoQuant) line(p1, p2 Point, config *LineConfig) {
	if config == nil {
		config = &LineConfig{}
	}

	if config.Location == "" {
		config.Location = "candle_pane"
	}

	g.lineStorage = append(g.lineStorage, LineData{Type: Segment, Config: *config, Points: []Point{p1, p2}})
}

func (g *GoQuant) plot(value float64, config *PlotConfig, label ...string) {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	} else {
		_, _, line, _ := runtime.Caller(1)
		lbl = fmt.Sprint(line)
	}

	if config == nil {
		config = &PlotConfig{}
	}

	if config.Location == "" {
		config.Location = lbl
	}

	var _value *float64
	if serie.NA(value) {
		_value = nil
	} else {
		_value = &value
	}

	if _, exists := g.plotStorage[lbl]; !exists {
		g.plotStorage[lbl] = PlotData{Config: *config, Data: []PlotPoint{}}
	}

	index := g.loopIndex + config.Shift
	if index < 0 || index >= len(g.bars) {
		return
	}

	plot := PlotPoint{Value: _value, Index: index, Time: int(g.bars[index].Time)}

	data := g.plotStorage[lbl].Data
	data = append(data, plot)
	g.plotStorage[lbl] = PlotData{Data: data, Config: *config}
}

// FillTheGaps will ensure every bar has a corresponding PlotPoint entry
func (g *GoQuant) FillTheGaps() {
	for label, plotData := range g.plotStorage {
		var filledData []PlotPoint
		barsLength := len(g.bars)
		plotLength := len(plotData.Data)

		if plotLength == 0 {
			continue
		}

		firstPlotIndex := plotData.Data[0].Index
		lastPlotIndex := plotData.Data[plotLength-1].Index

		// Handle missing plots at the beginning by adding nil values
		for i := 0; i < firstPlotIndex; i++ {
			filledData = append(filledData, PlotPoint{
				Value: nil,
				Index: i,
				Time:  int(g.bars[i].Time),
			})
		}

		// Start by adding the first plot point
		filledData = append(filledData, plotData.Data[0])

		// Loop through the plots and fill gaps
		for i := 1; i < plotLength; i++ {
			prevPoint := plotData.Data[i-1]
			currPoint := plotData.Data[i]

			// Check for a gap between prevPoint.Index and currPoint.Index
			gapSize := currPoint.Index - prevPoint.Index - 1

			if gapSize > 0 {
				// Interpolate across the gap
				for j := 1; j <= gapSize; j++ {
					interpolatedIndex := prevPoint.Index + j
					interpolatedTime := prevPoint.Time + (currPoint.Time-prevPoint.Time)*j/(gapSize+1)

					// Perform interpolation for values, handling nil cases
					var interpolatedValue *float64
					if prevPoint.Value != nil && currPoint.Value != nil {
						// Both values are non-nil, perform regular interpolation
						interpolated := *prevPoint.Value + (*currPoint.Value-*prevPoint.Value)*float64(j)/float64(gapSize+1)
						interpolatedValue = &interpolated
					} else if prevPoint.Value == nil && currPoint.Value != nil {
						// Prev is nil, use the current value for interpolation
						interpolated := *currPoint.Value
						interpolatedValue = &interpolated
					} else if prevPoint.Value != nil && currPoint.Value == nil {
						// Current is nil, use the previous value for interpolation
						interpolated := *prevPoint.Value
						interpolatedValue = &interpolated
					} else {
						// Both are nil, use nil for the interpolated value
						interpolatedValue = nil
					}

					// Append interpolated plot point
					filledData = append(filledData, PlotPoint{
						Value: interpolatedValue,
						Index: interpolatedIndex,
						Time:  interpolatedTime,
					})
				}
			}

			// Add current plot point
			filledData = append(filledData, currPoint)
		}

		// Handle missing plots at the end by adding nil values
		for i := lastPlotIndex + 1; i < barsLength; i++ {
			filledData = append(filledData, PlotPoint{
				Value: nil,
				Index: i,
				Time:  int(g.bars[i].Time),
			})
		}

		// Replace the old plot data with the filled data
		g.plotStorage[label] = PlotData{
			Data:   filledData,
			Config: plotData.Config,
		}
	}
}

func (g *GoQuant) RemoveDuplicatesFromStraightLines() {
	seen := make(map[string]bool)
	var uniqueLines []LineData

	for _, line := range g.lineStorage {
		var key string

		if line.Type == HorizontalStraightLine {
			key = fmt.Sprintf("%v|%v|%v", line.Type, line.Config, line.Points[0].Y)
		} else if line.Type == VerticalStraightLine {
			key = fmt.Sprintf("%v|%v|%v", line.Type, line.Config, line.Points[0].X)
		} else {
			continue
		}

		if _, exists := seen[key]; !exists {
			seen[key] = true
			uniqueLines = append(uniqueLines, line)
		}
	}

	g.lineStorage = uniqueLines
}

func (g *GoQuant) Logic(userFunc func(open, high, close, low, volume, time serie.Serie, ta TA, plot PlotF, line LineF, vline VLineF, hline HLineF)) {
	ta := TA{
		Cross:      g.cross,
		CrossOver:  g.crossOver,
		CrossUnder: g.crossUnder,
		BarsSince:  g.barsSince,
		ValueWhen:  g.valueWhen,
		PivotHigh:  g.pivotHigh,
		PivotLow:   g.pivotLow,
		Divergence: g.divergence,

		EMA:  g.ema,
		ATR:  g.atr,
		SMA:  g.sma,
		VWMA: g.vwma,
		RMA:  g.rma,
		RSI:  g.rsi,
	}

	plot := g.plot
	line := g.line
	vline := g.vline
	hline := g.hline

	endIndex := len(g.bars)
	for ; g.loopIndex < endIndex; g.loopIndex++ {
		userFunc(g.open, g.high, g.close, g.low, g.volume, g.time, ta, plot, line, vline, hline)
	}

	g.FillTheGaps()
	g.RemoveDuplicatesFromStraightLines()
}

func (g *GoQuant) Server(port int) error {
	http.HandleFunc("/bars", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jsonData, err := json.Marshal(serie.ConvertToPointerBars(g.bars))
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Error converting to JSON", http.StatusInternalServerError)
			return
		}

		w.Write(jsonData)
	})

	http.HandleFunc("/plots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jsonData, err := json.Marshal(g.plotStorage)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Error converting to JSON", http.StatusInternalServerError)
			return
		}

		w.Write(jsonData)
	})

	http.HandleFunc("/lines", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jsonData, err := json.Marshal(g.lineStorage)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Error converting to JSON", http.StatusInternalServerError)
			return
		}

		w.Write(jsonData)
	})

	distSubFS, err := fs.Sub(assets.Dist, "chart/dist")
	if err != nil {
		return err
	}

	http.Handle("/", http.FileServer(http.FS(distSubFS)))

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		return err
	}

	return nil
}
