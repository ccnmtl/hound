package main

type alertData struct {
	Name        string
	Metric      string
	Type        string
	Threshold   float64
	Direction   string
	EmailTo     string
	RunBookLink string
}

type configData struct {
	Alerts []alertData
}
