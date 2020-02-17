package server

import (
	"encoding/json"
	"fmt"
	"github.com/angelorc/go-uploader/models"
	"github.com/angelorc/go-uploader/services"
	"github.com/angelorc/go-uploader/transcoder"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"

	_ "github.com/angelorc/go-uploader/server/docs"
	"github.com/gorilla/mux"
	httpswagger "github.com/swaggo/http-swagger"
)

const (
	methodGET  = "GET"
	methodPOST = "POST"

	MAX_AUDIO_LENGTH = 610
)

// RegisterRoutes registers all HTTP routes with the provided mux router.
func RegisterRoutes(r *mux.Router, q chan *transcoder.Transcoder) {
	r.PathPrefix("/swagger/").Handler(httpswagger.WrapHandler)

	r.HandleFunc("/api/v1/upload/audio", uploadAudioHandler(q)).Methods(methodPOST)
	r.HandleFunc("api/v1/upload/image", uploadImageHandler()).Methods(methodPOST)

	r.HandleFunc("/api/v1/transcode/{id}", getTranscodeHandler()).Methods(methodGET)
}

type UploadAudioResp struct {
	Id       string  `json:"id"`
	FileName string  `json:"file_name"`
	Duration float32 `json:"duration"`
}

// @Summary Upload and transcode audio file
// @Description Upload, transcode and publish to ipfs an audio
// @Tags upload
// @Produce json
// @Param file formData file true "Transcoder file"
// @Success 200 {object} server.UploadAudioResp
// @Failure 400 {object} server.ErrorResponse "Error"
// @Router /upload/audio [post]
func uploadAudioHandler(q chan *transcoder.Transcoder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("file field is required"))
			return
		}
		defer file.Close()

		log.Info().Str("filename", header.Filename).Msg("handling new upload...")

		uploader := services.NewUploader(file, header)

		// check if the file is audio
		log.Info().Str("filename", header.Filename).Msg("check if the file is audio")

		if !uploader.IsAudio() {
			log.Error().Str("content-type", uploader.GetContentType()).Msg("Wrong content type")

			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("Wrong content type: %s", uploader.GetContentType()))
			return
		}

		// save original file
		_, err = uploader.SaveOriginal()
		log.Info().Str("filename", header.Filename).Msg("file save original")

		if err != nil {
			log.Error().Str("filename", uploader.Header.Filename).Msg("Cannot save audio file.")

			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("Cannot save audio file %s", uploader.Header.Filename))
			return
		}

		// check file size
		// check duration
		tm := models.NewTranscoder()
		if err := tm.Create(); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		audio := transcoder.NewTranscoder(uploader, tm.ID)
		log.Info().Str("filename", header.Filename).Msg("check audio duration")

		duration, err := audio.GetDuration()
		if err != nil {
			log.Error().Str("filename", uploader.Header.Filename).Msg("Cannot get audio duration.")

			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("Cannot get audio duration"))
			return
		}

		if duration > MAX_AUDIO_LENGTH {
			log.Error().Float32("duration", duration).Msg("File length is too big")

			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("File length is too big"))
			return
		}

		// transcode audio
		log.Info().Str("filename", header.Filename).Msg("transcode audio")

		q <- audio

		res := UploadAudioResp{
			Id: tm.ID.Hex(),
			FileName: uploader.Header.Filename,
			Duration: duration,
		}

		bz, err := json.Marshal(res)
		if err != nil {
			log.Error().Str("filename", uploader.Header.Filename).Msg("Failed to encode response")

			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("failed to encode response: %w", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bz)
	}
}

// @Summary Upload and create image file
// @Description Upload, create and publish to ipfs an image
// @Tags upload
// @Produce json
// @Param file formData file true "Image file"
// @Success 200 {object} server.UploadAudioResp
// @Failure 400 {object} server.ErrorResponse "Error"
// @Router /upload/image [post]
func uploadImageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not implemented"))
	}
}

// @Summary Get transcode status
// @Description Get transcode status by ID.
// @Tags transcode
// @Produce json
// @Param id path string true "ID"
// @Success 200 {object} models.Transcoder
// @Failure 400 {object} server.ErrorResponse "Failure to parse the id"
// @Failure 404 {object} server.ErrorResponse "Failure to find the id"
// @Router /transcode/{id} [get]
func getTranscodeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var params = mux.Vars(r)
		id := params["id"]

		pid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("cannot decode id"))
			return
		}

		tm := &models.Transcoder{
			ID:         pid,
		}

		res, err := tm.Get()
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Errorf("id not found"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}
