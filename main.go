package main

import (
	"fmt"
	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

func getIpfsShell() (*shell.Shell, bool) {
	sh := shell.NewShell("https://ipfs.infura.io:5001")
	if !sh.IsUp() {
		return nil, false
	}

	return sh, true
}

func saveOriginalAudio(file multipart.File, fileExt string) (*os.File, bool) {
	tmpDir := TMPPATH + "1/" // TODO: change with uniq id

	// create tmp dir
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		err = os.MkdirAll(tmpDir, 0755)
		if err != nil {
			log.Print(err)

			return nil, false
		}
	}

	tmpFile := tmpDir + "original" + fileExt

	// save file
	buff, err := os.Create(tmpFile)
	if err != nil {
		log.Print(err)

		return nil, false
	}
	//defer os.Remove(tmpFile)

	// write the content from POST to the file
	_, err = io.Copy(buff, file)
	if err != nil {
		log.Print(err)

		return nil, false
	}

	return buff, true
}

func checkAudioSize(buff *os.File) error {
	rawFile, err := ioutil.ReadAll(buff)
	if err != nil {
		log.Print(err)

		return fmt.Errorf("cannot read file")
	}


	// check audio size
	if len(rawFile) > MAX_AUDIO_FILE_SIZE {
		return fmt.Errorf(fmt.Sprintf(
			"File is too big, actual: %d, max: %d",
			len(rawFile),
			MAX_AUDIO_FILE_SIZE,
		))
	}

	return nil
}

// receiveHandler accepts the file and saves it to the current working directory
func receiveHandler(w http.ResponseWriter, r *http.Request) {

	_, ok := getIpfsShell()
	if !ok {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("IPFS node is down"),
		)

		return
	}

	// the FormFile function takes in the POST input id file
	file, audio, err := r.FormFile("file")
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("We cannot find upload file inside file field"),
		)

		return
	}
	defer file.Close()

	// check if the content type is allowed
	contentType := audio.Header.Get("Content-Type")
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

	// save original file
	fileExt := filepath.Ext(audio.Filename)
	buffer, ok := saveOriginalAudio(file, fileExt)
	if !ok {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot save audio file %s", audio.Filename),
			),
		)
	}

	err = checkAudioSize(buffer)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(
				err.Error(),
			),
		)

		return
	}

	defer buffer.Close()

	/*

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
				fmt.Sprintf("Cannot process audio file %s", audio.Filename),
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
				fmt.Sprintf("Error when parse %s", audio.Filename),
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

	uploadSession := "1"

	fileExt := filepath.Ext(audio.Filename)
	fileName := fmt.Sprintf("%s_%s.mp3", uploadSession, strings.TrimRight(audio.Filename, fileExt))

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
				fmt.Sprintf("Cannot process audio file %s", audio.Filename),
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

	// Split file into segments
	cmd = exec.Command(
		"ffmpeg",
		"-i",
		"./tmp/"+fileName,
		"-f",
		"segment",
		"-segment_time",
		"5", // 5sec
		"-c",
		"copy",
		"./tmp/" + uploadSession + "%03d.mp3",
	)

	cmd.Stderr = &ffmpegStdErr

	err = cmd.Run()
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot split audio into segments file %s", audio.Filename),
			),
		)

		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return
	}



	log.Print("ok")
	*/


	// add and pin files to ipfs
	/*cid, err := ipfsShell.Add(buff)
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot upload segment to ipfs %s", audio.Filename),
			),
		)

		return
	}


	// remove converted tmp files
	// send publish tx to bitsong


	writeJSONResponse(
		w,
		http.StatusCreated,
		fmt.Sprintf("cid %v", cid),
	)*/
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
	log.Fatal(http.ListenAndServe(":8081", nil))
}