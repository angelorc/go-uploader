package services

import (
	"github.com/google/uuid"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

type Uploader struct {
	ID     uuid.UUID
	File   multipart.File
	Header *multipart.FileHeader
}

func NewUploader(file multipart.File, header *multipart.FileHeader) *Uploader {
	id, err := uuid.NewUUID()
	if err != nil {
		panic("cannot generate new uuid")
	}

	return &Uploader{
		ID:     id,
		File:   file,
		Header: header,
	}
}

func (u *Uploader) GetID() string {
	return u.ID.String()
}

func (u *Uploader) GetContentType() string {
	return u.Header.Header.Get("Content-Type")
}

func (u *Uploader) GetExtension() string {
	return filepath.Ext(u.Header.Filename)
}

func (u *Uploader) IsAudio() bool {
	contentType := u.GetContentType()
	return contentType == "audio/aac" || contentType == "audio/wav" || contentType == "audio/mp3" || contentType == "application/octet-stream"
}

func (u *Uploader) GetDir() string {
	return ".bitsongms/uploader/" + u.GetID() + "/"
}

func (u *Uploader) GetTmpOriginalFileName() string {
	return u.GetDir() + "original" + u.GetExtension()
}

func (u *Uploader) GetTmpConvertedFileName() string {
	return u.GetDir() + "converted" + u.GetExtension()
}

func (u *Uploader) createDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u *Uploader) SaveOriginal() (*os.File, error) {
	// create tmp dir
	if err := u.createDir(u.GetDir()); err != nil {
		return nil, err
	}

	// save file
	buff, err := os.Create(u.GetTmpOriginalFileName())
	if err != nil {
		return nil, err
	}

	// write the content from POST to the file
	_, err = io.Copy(buff, u.File)
	if err != nil {
		return nil, err
	}

	return buff, nil
}
