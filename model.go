package main

type FFProbeFormat struct {
	StreamsCount int32   `json:"nb_streams"`
	Format       string  `json:"format_name"`
	Duration     float32 `json:"duration,string"`
}

type AudioFFProbe struct {
	Format FFProbeFormat `json:"format"`
}