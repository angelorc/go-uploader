package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// 100 Mb - max raw audio size that we allow upload
	MAX_AUDIO_FILE_SIZE = 1024 * 1024 * 1

	// 100 Mb - max original audio size that we store
	MAX_ORIGINAL_FILE_SIZE = 1024 * 1024 * 100

	// 10 min + 10 sec buff
	MAX_AUDIO_LENGTH = 610

	TMPPATH = "./tmp/"
)

func isAudioContentType(contentType string) bool {
	// application/octet-stream - binary file without format, some encoders can record audio without mime type
	return contentType == "audio/aac" || contentType == "audio/wav" || contentType == "audio/mp3"  || contentType == "application/octet-stream"
}

// infoHandler returns an HTML upload form
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		fmt.Fprintf(w, `<html>
<head>
  <title>GoLang HTTP Fileserver</title>
</head>
<body>
<h2>Upload a file</h2>
<form action="/receive" method="post" enctype="multipart/form-data">
  <label for="file">Filename:</label>
  <input type="file" name="file" id="file">
  <br>
  <input type="submit" name="submit" value="Submit">
</form>
</body>
</html>`)
	}
}

// receiveHandler accepts the file and saves it to the current working directory
func receiveHandler(w http.ResponseWriter, r *http.Request) {

	// the FormFile function takes in the POST input id file
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("We cannot find upload file inside file field"),
		)

		log.Print(err)

		return
	}
	defer file.Close()

	// check if the content type is allowed
	contentType := header.Header.Get("Content-Type")
	if !isAudioContentType(contentType) {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Wrong content type: %s", contentType),
			),
		)

		return
	}

	tmpFile := TMPPATH + header.Filename

	// save file
	buff, err := os.Create(tmpFile)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot create audio file %s", header.Filename),
			),
		)

		log.Print(err)

		return
	}
	defer buff.Close()
	defer os.Remove(tmpFile)

	// write the content from POST to the file
	_, err = io.Copy(buff, file)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot save audio file %s", header.Filename),
			),
		)

		log.Print(err)

		return
	}

	// get rawFile
	rawFile, err := ioutil.ReadAll(buff)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Cannot read file %s", header.Filename),
			),
		)

		log.Print(err)

		return
	}

	// check audio size
	if len(rawFile) > MAX_AUDIO_FILE_SIZE {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(rawFile),
					MAX_AUDIO_FILE_SIZE,
				),
			),
		)

		return
	}

	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-i",
		tmpFile,
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

	err = cmd.Run()
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot process audio file %s", header.Filename),
			),
		)

		log.Print("FFProbe error ", err)
		log.Print(string(ffprobeStdErr.Bytes()))

		return
	}

	ffprobeOutput := ffprobeStdOut.Bytes()
	ffprobeResult := AudioFFProbe{}

	err = json.Unmarshal(ffprobeOutput, &ffprobeResult)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Error when parse %s", header.Filename),
			),
		)

		log.Print(err)
		log.Print(string(ffprobeOutput))

		return
	}

	if ffprobeResult.Format.Duration > MAX_AUDIO_LENGTH {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("File length is too big"),
		)

		return
	}

	fileExt := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("converted_%s.mp3", strings.TrimRight(header.Filename, fileExt))

	cmd = exec.Command(
		"ffmpeg",
		"-i",
		tmpFile,
		"-acodec",
		"libmp3lame",
		"-ab",
		"128k",
		"-metadata",
		"artist=ownerAddress",
		"-y",
		"./tmp/"+fileName,
	)

	var ffmpegStdErr bytes.Buffer
	cmd.Stderr = &ffmpegStdErr

	err = cmd.Run()
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot process audio file %s", header.Filename),
			),
		)

		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return
	}

	formattedFile, err := ioutil.ReadFile("./tmp/" + fileName)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot read file %s after proccesing", "./tmp/"+fileName),
			),
		)

		log.Print(err)

		return
	}

	if len(formattedFile) > MAX_ORIGINAL_FILE_SIZE {
		defer os.Remove("./tmp/" + fileName)

		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(formattedFile),
					MAX_ORIGINAL_FILE_SIZE,
				),
			),
		)

		return
	}


	writeJSONResponse(
		w,
		http.StatusCreated,
		fmt.Sprintf("Duration %v", ffprobeResult.Format.Duration),
	)
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("err=", err)
		os.Exit(1)
	}

	http.HandleFunc("/upload", uploadHandler)   // Display a form for user to upload file
	http.HandleFunc("/receive", receiveHandler) // Handle the incoming file
	http.Handle("/", http.FileServer(http.Dir(dir)))
	log.Fatal(http.ListenAndServe(":8080", nil))
}