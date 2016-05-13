package main

type AlertData struct {
	Name        string
	Metric      string
	Type        string
	Threshold   float64
	Direction   string
	EmailTo     string
	RunBookLink string
}

type ConfigData struct {
	Alerts []AlertData
}
