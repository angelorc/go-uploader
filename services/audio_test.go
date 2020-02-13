package services

import (
	"fmt"
	"testing"
)

func TestAudio_GetDuration(t *testing.T) {
	audio := NewAudio("/home/angelo/Progetti/go-uploader/tmp/4291c8c9-4e74-11ea-b78e-08606e8419b0/original.mp3")
	duration, err := audio.GetDuration()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(duration)

	duration, err = audio.GetDuration()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(duration)
}
