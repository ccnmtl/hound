package main

type AlertData struct {
	Name      string
	Metric    string
	Threshold float64
	Direction string
}

type ConfigData struct {
	GraphiteBase   string
	EmailFrom      string
	EmailTo        string
	CheckInterval  int
	GlobalThrottle int
	Alerts         []AlertData
}
