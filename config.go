package main

type AlertData struct {
	Name      string
	Metric    string
	Threshold float64
	Direction string
}

type ConfigData struct {
	GraphiteBase   string
	CarbonBase     string
	MetricBase     string
	EmailFrom      string
	EmailTo        string
	CheckInterval  int
	GlobalThrottle int
	HttpPort       string
	TemplateFile   string
	Alerts         []AlertData
}
