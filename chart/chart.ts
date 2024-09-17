import klinecharts, {
  registerFigure,
  registerIndicator,
  registerOverlay,
  init,
  LineType,
} from "klinecharts";

interface Bar {
  close: Number;
  open: Number;
  high: Number;
  low: Number;
  volume: Number;
  timestamp: number;
}
interface PlotsData {
  [key: string]: {
    data: any[];
    config: PlotConfig;
  };
}
interface LineConfig {
  color?: string;
  width?: number;
  dashed?: number;
  location?: string;
  shift?: number;
}
interface Point {
  x: number;
  y: number;
}
interface LineData {
  type: "horizontalStraightLine" | "verticalStraightLine" | "segment";
  config: LineConfig;
  points: Point[];
}
interface PlotConfig {
  color?: string;
  width?: number;
  dashed?: number;
  smooth?: number;
  precision?: number;
  location?: string;
  shift?: number;
}

let colorI = 0;
const getColor = () => {
  const colors = [
    "#4169E1", // Royal Blue
    "#DC143C", // Crimson
    "#FF8C00", // Dark Orange
    "#3CB371", // Medium Sea Green
    "#FF1493", // Deep Pink
    "#FFD700", // Gold
    "#9400D3", // Dark Violet
    "#40E0D0", // Turquoise
    "#B22222", // Firebrick
    "#32CD32", // Lime Green
    "#4682B4", // Steel Blue
    "#FF6347", // Tomato
    "#6A5ACD", // Slate Blue
    "#BDB76B", // Dark Khaki
    "#008080", // Teal
    "#F4A460", // Sandy Brown
    "#9370DB", // Medium Purple
    "#2E8B57", // Sea Green
    "#E9967A", // Dark Salmon
    "#1E90FF", // Dodger Blue
  ];
  if (colorI >= colors.length) {
    colorI = 0;
  }

  return colors[colorI++];
};

function sortBars(_bars: Bar[]): Bar[] {
  if (_bars.length > 1 && _bars[0].timestamp > _bars[1].timestamp) {
    _bars.reverse();
  }

  if (_bars.length > 0 && _bars[0].timestamp < 1e10) {
    _bars.forEach((bar) => {
      bar.timestamp *= 1000;
    });
  }

  return _bars;
}

export async function Init(element: string | HTMLElement) {
  const chart = init(element, { timezone: "UTC" })!;
  console.log("Inited");

  const [response1, response2, response3] = await Promise.all([
    fetch("http://localhost:3000/bars"),
    fetch("http://localhost:3000/plots"),
    fetch("http://localhost:3000/lines"),
  ]);

  const bars = (await response1.json()) as Bar[];
  const plots = (await response2.json()) as PlotsData;
  const lines = (await response3.json()) as LineData[];

  chart.applyNewData(sortBars(bars) as klinecharts.KLineData[]);

  applyIndicators(chart, plots);

  if (lines && lines.length > 0) {
    applyLines(chart, lines);
  }
  return chart;
}

function applyIndicators(chart: klinecharts.Chart, plots: PlotsData) {
  const organizedPlots = Object.values(plots).reduce((acc, plot) => {
    const location = plot.config.location || "pane_1"; // default to oscillator if no location is specified
    if (!acc[location]) {
      acc[location] = [];
    }
    acc[location].push(plot);
    return acc;
  }, {} as Record<string, { data: any[]; config: PlotConfig }[]>);

  console.log(organizedPlots);
  //
  Object.entries(organizedPlots).forEach(([location, plots]) => {
    const figures = plots.map((plot, i) => ({
      key: `line_${i + 1}`,
      type: "line",
      title: `line${i + 1}:`,
    }));

    registerIndicator({
      name: location,
      shortName: location,
      calcParams: [],
      precision: plots[0].config.precision || 1,
      figures: figures,
      styles: {
        lines: plots.map((plot) => ({
          size: plot.config.width || 0,
          color: plot.config.color || getColor(),
          smooth: plot.config.smooth || 0,
          style: plot.config.dashed ? LineType.Dashed : LineType.Solid,
          dashedValue: plot.config.dashed ? [plot.config.dashed] : [],
        })),
      },
      calc: (kLineDataList) => {
        const it = kLineDataList.map((kLineData, i) => {
          const data: { [key: string]: any } = {};

          for (let j = 0; j < plots.length; j++) {
            const value = plots[j].data[i].value;
            data[`line_${j + 1}`] = value === null ? NaN : value;
          }

          return data;
        });
        console.log("it:")
        console.log(it);
        return it;
      },
    });

    console.log(`id=${location}`);
    chart!.createIndicator(
      location,
      true,
      {
        id: location,
        height: location === "candle_pane" ? undefined : 100,
        minHeight: 30,
        dragEnabled: true,
        gap: { top: 0.2, bottom: 0.1 },
        axisOptions: { scrollZoomEnabled: true },
      },
      () => {}
    );
  });
}

function applyLines(chart: klinecharts.Chart, lines: LineData[]) {
  for (const line of lines) {
    chart.createOverlay(
      {
        name: line.type,
        points: line.points.map((l) => ({ timestamp: l.x * 1000, value: l.y })),
        styles: {
          line: {
            color: line.config.color || getColor(),
            style: line.config.dashed ? LineType.Dashed : LineType.Solid,
            dashedValue: line.config.dashed ? [line.config.dashed] : [],
            size: line.config.width,
          },
        },
      },
      line.config.location
    );
  }
}
