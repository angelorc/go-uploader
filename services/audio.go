package services

import (
	"bytes"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FFProbeFormat struct {
	ready        bool
	StreamsCount int32   `json:"nb_streams"`
	Format       string  `json:"format_name"`
	Duration     float32 `json:"duration,string"`
}

type Audio struct {
	Uploader *Uploader
	Format   FFProbeFormat `json:"format"`
}

func NewAudio(u *Uploader) *Audio {
	return &Audio{
		Uploader: u,
		Format: FFProbeFormat{
			ready: false,
		},
	}
}

func (a *Audio) SplitToSegments() error {
	newName := a.Uploader.getDir() + "segment%03d.ts"
	m3u8FileName := a.Uploader.getDir() + "list.m3u8"

	cmd := exec.Command(
		"ffmpeg",
		"-i", a.Uploader.GetTmpConvertedFileName(),
		"-ar", "48000", // sample rate
		"-b:a", "320k", // bitrate
		"-hls_time", "5", // 5s for each segment
		"-hls_segment_type", "mpegts", // hls segment type: Output segment files in MPEG-2 Transport Stream format. This is compatible with all HLS versions.
		"-hls_list_size", "0", //  If set to 0 the list file will contain all the segments
		//"-hls_base_url", "segments/",
		"-hls_segment_filename", newName,
		"-vn", m3u8FileName,
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

type AudioSegment struct {
	Path   string
	Format FFProbeFormat `json:"format"`
}

type AudioSegments []*AudioSegment

func NewAudioSegment(path string) *AudioSegment {
	return &AudioSegment{
		Path: path,
		Format: FFProbeFormat{
			ready: false,
		},
	}
}

func (as *AudioSegment) ffprobe() error {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-i",
		as.Path,
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
		return err
	}

	ffprobeOutput := ffprobeStdOut.Bytes()
	as.Format = FFProbeFormat{
		ready: true,
	}

	err = json.Unmarshal(ffprobeOutput, &as)
	if err != nil {
		return err
	}

	return nil
}

func (as *AudioSegment) GetDuration() (float32, error) {
	if !as.Format.ready {
		err := as.ffprobe()
		if err != nil {
			return float32(0), err
		}

		return as.Format.Duration, nil

	}

	return as.Format.Duration, nil
}

func (a *Audio) GetSegments() (AudioSegments, error) {
	var segments AudioSegments

	err := filepath.Walk(a.Uploader.getDir(), func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".ts") {
			segment := &AudioSegment{
				Path: "./" + path,
			}
			segments = append(segments, segment)
		}

		return nil
	})

	return segments, err
}

func (a *Audio) RemoveFiles() error {
	if err := os.Remove(a.Uploader.GetTmpOriginalFileName()); err != nil {
		return err
	}

	if err := os.Remove(a.Uploader.GetTmpConvertedFileName()); err != nil {
		return err
	}

	return nil
}

func (a *Audio) TranscodeToMp3() error {
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		a.Uploader.GetTmpOriginalFileName(),
		"-acodec",
		"libmp3lame",
		"-ar",
		"48000",
		"-b:a",
		"320k",
		"-y",
		a.Uploader.GetTmpConvertedFileName(),
	)

	var ffmpegStdErr bytes.Buffer
	cmd.Stderr = &ffmpegStdErr

	err := cmd.Run()
	if err != nil {
		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return err
	}

	_, err = ioutil.ReadFile(a.Uploader.GetTmpConvertedFileName())
	if err != nil {
		return err
	}

	return nil
}

func (a *Audio) ffprobe() error {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-i",
		a.Uploader.GetTmpOriginalFileName(),
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
		return err
	}

	ffprobeOutput := ffprobeStdOut.Bytes()
	a.Format = FFProbeFormat{
		ready: true,
	}

	err = json.Unmarshal(ffprobeOutput, &a)
	if err != nil {
		return err
	}

	return nil
}

func (a *Audio) GetDuration() (float32, error) {
	if !a.Format.ready {
		err := a.ffprobe()
		if err != nil {
			return float32(0), err
		}

		return a.Format.Duration, nil

	}

	return a.Format.Duration, nil
}
