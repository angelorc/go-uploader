package services

import (
	"bytes"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

type FFProbeFormat struct {
	ready        bool
	StreamsCount int32   `json:"nb_streams"`
	Format       string  `json:"format_name"`
	Duration     float32 `json:"duration,string"`
}

type Audio struct {
	Path   string
	Format FFProbeFormat `json:"format"`
}

func NewAudio(path string) *Audio {
	return &Audio{
		Path: path,
		Format: FFProbeFormat{
			ready: false,
		},
	}
}

func (a *Audio) SplitToSegments() error {
	fileConverted := strings.Replace(a.Path, "original", "converted", 1)
	newName := strings.Replace(fileConverted, "converted", "%03d", 1)

	cmd := exec.Command(
		"ffmpeg",
		"-i",
		fileConverted,
		"-f",
		"segment",
		"-segment_time",
		"5", // 5sec
		"-c",
		"copy",
		newName,
	)

	var ffmpegStdErr bytes.Buffer
	cmd.Stderr = &ffmpegStdErr

	err := cmd.Run()
	if err != nil {
		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return err
	}

	return nil
}

func (a *Audio) RemoveOriginal() error {
	fileConverted := strings.Replace(a.Path, "original", "converted", 1)

	if err := os.Remove(a.Path); err != nil {
		return err
	}

	if err := os.Remove(fileConverted); err != nil {
		return err
	}

	return nil
}

func (a *Audio) ConvertToMp3() error {
	newName := strings.Replace(a.Path, "original", "converted", 1)

	cmd := exec.Command(
		"ffmpeg",
		"-i",
		a.Path,
		"-acodec",
		"libmp3lame",
		"-ab",
		"128k",
		"-y",
		newName,
	)

	var ffmpegStdErr bytes.Buffer
	cmd.Stderr = &ffmpegStdErr

	err := cmd.Run()
	if err != nil {
		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return err
	}

	_, err = ioutil.ReadFile(newName)
	if err != nil {
		return err
	}

	return nil
}

func (a *Audio) GetDuration() (float32, error) {
	if !a.Format.ready {
		cmd := exec.Command(
			"ffprobe",
			"-v",
			"error",
			"-i",
			a.Path,
			"-print_format",
			"json",
			"-show_format",
		)

		var (
			// There are some uneeded information inside StdOut, skip it
			ffprobeStdOut bytes.Buffer
			ffprobeStdErr bytes.Buffer
		)

		cmd.Stdout = &ffprobeStdOut
		cmd.Stderr = &ffprobeStdErr

		err := cmd.Run()
		if err != nil {
			return float32(0), err
		}

		ffprobeOutput := ffprobeStdOut.Bytes()
		ffprobeResult := Audio{
			Path: a.Path,
			Format: FFProbeFormat{
				ready: true,
			},
		}

		err = json.Unmarshal(ffprobeOutput, &ffprobeResult)
		if err != nil {
			return float32(0), err
		}

		*a = ffprobeResult

		return a.Format.Duration, nil

	}

	return a.Format.Duration, nil
}
