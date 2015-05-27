package main

type AlertData struct {
	Name      string
	Metric    string
	Threshold float64
	Direction string
	EmailTo   string
}

type ConfigData struct {
	Alerts []AlertData
}
