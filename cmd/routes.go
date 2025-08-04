package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fileserve"
	"fileserve/store"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"sync"
	"time"
)

type RouteHandler struct {
	fileStore store.FileStore
	pool      sync.Pool
}

func NewRoutHandler(fs store.FileStore) *RouteHandler {
	return &RouteHandler{fileStore: fs, pool: sync.Pool{New: newBuffer}}
}

func (r *RouteHandler) AddFile(resp http.ResponseWriter, req *http.Request) {
	mpfile, mpfileHeader, err := req.FormFile("f")
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	defer mpfile.Close()

	md5Hash, buffer, err := r.hashContent(mpfile)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.pool.Put(buffer)

	contentType := mime.TypeByExtension(path.Ext(mpfileHeader.Filename))

	slog.Info("adding file", "hash", md5Hash, "filename", mpfileHeader.Filename)

	fd := fileserve.FileData{
		FileMetadata: fileserve.FileMetadata{
			Name:        mpfileHeader.Filename,
			Size:        mpfileHeader.Size,
			Hash:        md5Hash,
			ContentType: contentType},
		Data: buffer.Bytes(),
	}

	metadata, err := r.fileStore.AddFile(fd)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(metadata)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	_, err = io.Copy(resp, bytes.NewReader(b))
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (r *RouteHandler) GetFile(resp http.ResponseWriter, req *http.Request) {
	id := req.PathValue("hash")
	if len(id) == 0 {
		resp.WriteHeader(http.StatusBadRequest)
	}

	file, err := r.fileStore.GetFile(id)
	if processError(err, resp) {
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	buffer := bytes.NewReader(file.Data)
	http.ServeContent(resp, req, file.Name, time.Now(), buffer)
}

func (r *RouteHandler) DeleteFile(resp http.ResponseWriter, req *http.Request) {
	hash := req.PathValue("hash")
	err := r.fileStore.DeleteFile(hash)
	processError(err, resp)
}

func (r *RouteHandler) hashContent(in io.ReadCloser) (string, *bytes.Buffer, error) {
	buffer := r.pool.Get().(*bytes.Buffer)
	_, err := io.Copy(buffer, in)
	if err != nil {
		return "", nil, err
	}

	sum := md5.Sum(buffer.Bytes())
	dst := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(dst, sum[:])
	return string(dst), buffer, nil
}

func registerHandlers(h *RouteHandler) {
	http.HandleFunc("POST /files", h.AddFile)
	http.HandleFunc("DELETE /files/{hash}", h.DeleteFile)
	//http.HandleFunc("GET /files", h.ListFiles)
	http.HandleFunc("GET /files/{hash}", h.GetFile)

}

func processError(err error, resp http.ResponseWriter) bool {
	code := http.StatusInternalServerError

	if err == nil {
		return false
	} else {
		var ce fileserve.CodedHttpError
		if errors.As(err, &ce) {
			code = ce.HttpCode()
		} else {
			slog.Error("unhandled error type", "err", fmt.Sprintf("%T - %s", err, err.Error()))
		}
	}

	resp.WriteHeader(code)
	return true
}

func newBuffer() any {
	return new(bytes.Buffer)
}
