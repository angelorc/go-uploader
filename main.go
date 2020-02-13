package main

import (
	//"bytes"
	//"encoding/json"
	"fmt"
	"github.com/angelorc/go-uploader/services"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	//"os/exec"
	//"path/filepath"
	//"strings"
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

	ipfs := services.NewIpfs()
	if !ipfs.IsUp() {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("IPFS node is down"),
		)

		return
	}

	// the FormFile function takes in the POST input id file
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("We cannot find upload file inside file field"),
		)

		return
	}
	defer file.Close()

	upload := services.NewUploader(file, header)

	// check if the file is audio
	if !upload.IsAudio() {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Wrong content type: %s", upload.GetContentType()),
			),
		)
		return
	}

	// save original file
	_, err = upload.SaveOriginal()
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot save audio file %s", upload.Header.Filename),
			),
		)
	}

	// check file size
	// check duration
	audio := services.NewAudio(upload.GetTmpOriginalFileName())
	duration, err := audio.GetDuration()
	if err != nil {
		writeJSONResponse(
			w,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot get audio duration"),
			),
		)
		return
	}

	if duration > MAX_AUDIO_LENGTH {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson("File length is too big"),
		)

		return
	}

	// Convert to mp3
	if err := audio.ConvertToMp3(); err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(err.Error()),
		)

		return
	}

	// check size compared to original

	// spilt mp3 to segments
	if err := audio.SplitToSegments(); err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(err.Error()),
		)

		return
	}

	// remove original and converted
	if err := audio.RemoveOriginal(); err != nil {
		writeJSONResponse(
			w,
			http.StatusBadRequest,
			newErrorJson(err.Error()),
		)

		return
	}

	/*

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
