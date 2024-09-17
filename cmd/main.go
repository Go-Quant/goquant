package main

import (
	"fmt"

	gq "github.com/Go-Quant/goquant/core"
	"github.com/Go-Quant/goquant/serie"
)

var GQ *gq.GoQuant

func main() {
	port := 3000
	data, err := serie.ProcessBarsFromPath("./sample-data.json")
	if err != nil {
		panic(err)
	}

	GQ = gq.New()
	GQ.AddBars(data)
	GQ.Logic(myLogic)

	fmt.Printf("Server running at http://localhost:%d\n", port)
	panic(GQ.Server(port))
}

// shortcuts
type PlotConfig = gq.PlotConfig
type LineConfig = gq.LineConfig
type Point = gq.Point

func myLogic(open, high, close, low, volume, _time serie.Serie, ta gq.TA, plot gq.PlotF, line gq.LineF, vline gq.VLineF, hline gq.HLineF) {

	rsi := ta.RSI(close, 14).Get() // or .G(0)
	sma := ta.SMA(close, 14).Get() // or .G(0)

	plot(sma, &PlotConfig{Location: "candle_pane"})
	plot(rsi, nil, "rsi")

	// rsi 30, 50 and 70 lines
	hline(30, &LineConfig{Color: "#787B80", Width: 1, Dashed: 5, Location: "rsi"})
	hline(50, &LineConfig{Color: "#787B80", Width: .5, Dashed: 5, Location: "rsi"})
	hline(70, &LineConfig{Color: "#787B80", Width: 1, Dashed: 5, Location: "rsi"})
}
