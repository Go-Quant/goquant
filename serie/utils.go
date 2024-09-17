package serie

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

func NA(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0)
}

func NZ(value float64, replacement ...float64) float64 {
	rep := 0.0
	if len(replacement) > 0 {
		rep = replacement[0]
	}

	if NA(value) {
		return rep
	}

	return value
}

func OR(values ...float64) float64 {
	for _, value := range values {
		if !NA(value) {
			return value
		}
	}
	return math.NaN()
}

func ExtractField(bars []Bar, field string) []float64 {
	// Create a slice with the same length as bars
	values := make([]float64, len(bars))

	// Iterate through bars and extract the field value
	for i, bar := range bars {
		switch field {
		case "Close":
			values[i] = bar.Close
		case "Open":
			values[i] = bar.Open
		case "High":
			values[i] = bar.High
		case "Low":
			values[i] = bar.Low
		case "Volume":
			values[i] = bar.Volume
		case "Time":
			values[i] = bar.Time
		default:
			// Handle invalid field
			fmt.Printf("Unknown field: %s\n", field)
			return nil
		}
	}
	return values
}

func TimestampToFormattedDate(timestamp float64) string {
	t := time.Unix(int64(timestamp), 0)
	return t.Format("Mon 02 Jan '06")
}

// MapJsonToBar maps the JSON object with different field names to a Bar struct
func mapJsonToBar(jsonMap map[string]interface{}, fieldMap map[string]string) (Bar, error) {
	bar := Bar{}

	// Mapping user fields to Bar struct
	var err error
	for jsonKey, jsonValue := range jsonMap {
		switch fieldMap[jsonKey] {
		case "Close":
			bar.Close, err = toFloat(jsonValue)
			if err != nil {
				return Bar{}, err
			}
		case "Open":
			bar.Open, err = toFloat(jsonValue)
			if err != nil {
				return Bar{}, err
			}
		case "High":
			bar.High, err = toFloat(jsonValue)
			if err != nil {
				return Bar{}, err
			}
		case "Low":
			bar.Low, err = toFloat(jsonValue)
			if err != nil {
				return Bar{}, err
			}
		case "Volume":
			bar.Volume, err = toFloat(jsonValue)
			if err != nil {
				return Bar{}, err
			}
		case "Time":
			bar.Time, err = toFloat(jsonValue)
			if err != nil {
				return Bar{}, err
			}
		}
	}

	return bar, nil
}

// toFloat is a utility function to convert interface{} to float64
func toFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unsupported type for conversion to float64")
	}
}

// NormalizeTimestamps converts timestamps to seconds if they are in ms or ns
func normalizeTimestamps(bars []Bar) []Bar {
	for i := range bars {
		switch {
		case bars[i].Time > 1e12: // Nanoseconds to seconds
			bars[i].Time = bars[i].Time / 1e9
		case bars[i].Time > 1e10: // Milliseconds to seconds
			bars[i].Time = bars[i].Time / 1e3
		}
	}
	return bars
}

// FillMissingBars fills gaps between bars by adding zero-value Bars for missing timestamps
// It calculates the expected interval from the first two bars and returns an error if the data is not consistent
func fillBarGaps(bars []Bar) ([]Bar, error) {
	if len(bars) < 2 {
		return bars, nil // No need to fill gaps if there are fewer than 2 bars
	}

	// Calculate the expected time interval using the first two bars
	expectedInterval := bars[1].Time - bars[0].Time

	if expectedInterval <= 0 {
		return nil, fmt.Errorf("data error: invalid or zero interval between the first two bars")
	}

	NaN := math.NaN()
	var filledBars []Bar
	for i := 0; i < len(bars)-1; i++ {
		filledBars = append(filledBars, bars[i])
		timeDiff := bars[i+1].Time - bars[i].Time

		if timeDiff > expectedInterval {
			numMissingBars := int(math.Round(timeDiff/expectedInterval)) - 1
			for j := 1; j <= numMissingBars; j++ {
				// Create a zero-value Bar with the timestamp incremented
				filledBars = append(filledBars, Bar{Time: bars[i].Time + float64(j)*expectedInterval, Close: NaN, Open: NaN, High: NaN, Low: NaN, Volume: NaN})
			}
		} else if timeDiff < expectedInterval {
			return nil, fmt.Errorf("data error: unexpected time difference between bars: %.2f seconds", timeDiff)
		}
	}
	// Append the last Bar
	filledBars = append(filledBars, bars[len(bars)-1])

	return filledBars, nil
}

var fieldMap = map[string]string{
	"close":     "Close",
	"open":      "Open",
	"high":      "High",
	"low":       "Low",
	"volume":    "Volume",
	"time":      "Time", // User-provided 'time' maps to 'Time' in Bar struct
	"timestamp": "Time", // User-provided 'timestamp' maps to 'Time'
	"ts":        "Time", // User-provided 'ts' also maps to 'Time' in Bar struct
}

// ProcessBarsFromJson processes the JSON string input, handles dynamic field mapping, normalizes timestamps, fills missing bars with zero Bar, and checks if reversing is needed
func ProcessBarsFromJsonString(jsonStr string) ([]Bar, error) {
	// Step 1: Decode JSON into an array of maps
	var jsonObjects []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonObjects); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %v", err)
	}

	// Step 2: Map the JSON objects to the Bar struct using fieldMap
	var bars []Bar
	for _, jsonObj := range jsonObjects {
		bar, err := mapJsonToBar(jsonObj, fieldMap)
		if err != nil {
			return nil, fmt.Errorf("error mapping JSON to Bar: %v", err)
		}
		bars = append(bars, bar)
	}

	// Step 3: Normalize timestamps
	bars = normalizeTimestamps(bars)

	// Step 4: Check if data needs to be reversed based on the first two bars' timestamps
	if len(bars) >= 2 && bars[0].Time > bars[1].Time {
		// Reverse the array
		for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
			bars[i], bars[j] = bars[j], bars[i]
		}
	}

	// Step 5: Fill missing Bars
	bars, err := fillBarGaps(bars)
	if err != nil {
		return nil, fmt.Errorf("error filling bar gaps: %v", err)
	}

	return bars, nil
}

func ProcessBarsFromPath(path string) ([]Bar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", path, err)
	}

	bars, err := ProcessBarsFromJsonString(string(data))
	if err != nil {
		return nil, fmt.Errorf("error processing JSON data: %v", err)
	}

	return bars, nil
}

type BarPointer struct {
	Close  *float64 `json:"close"`
	Open   *float64 `json:"open"`
	High   *float64 `json:"high"`
	Low    *float64 `json:"low"`
	Volume *float64 `json:"volume"`
	Time   *float64 `json:"timestamp"`
}

func floatToPointerWithCheck(f float64) *float64 {
	if NA(f) {
		return nil
	}

	return &f
}

// Function to convert []Bar to []BarPointer
func ConvertToPointerBars(bars []Bar) []BarPointer {
	pointerBars := make([]BarPointer, len(bars))

	for i, bar := range bars {
		pointerBars[i] = BarPointer{
			Close:  floatToPointerWithCheck(bar.Close),
			Open:   floatToPointerWithCheck(bar.Open),
			High:   floatToPointerWithCheck(bar.High),
			Low:    floatToPointerWithCheck(bar.Low),
			Volume: floatToPointerWithCheck(bar.Volume),
			Time:   floatToPointerWithCheck(bar.Time),
		}
	}

	return pointerBars
}
