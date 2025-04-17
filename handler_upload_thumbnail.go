package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	filetype := header.Header.Get("Content-Type")

	Type, _, err := mime.ParseMediaType(filetype)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Mime parse error", err)
		return
	}

	if Type != "image/jpeg" && Type != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Inncorect file type. Expecting png of jpeg", nil)
		return
	}

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video not found file", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unnorthurized", err)
		return
	}
	fileTypeSp := strings.Split(filetype, "/")

	key := make([]byte, 32)
	rand.Read(key)
	url := base64.RawURLEncoding.EncodeToString(key)

	filePath := filepath.Join("assets", url) + "." + fileTypeSp[1]
	img, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File create error", err)
		return
	}
	io.Copy(img, file)
	thumbURL := "http://localhost:" + cfg.port + "/" + filePath

	updateVideoData := database.Video{
		ID:                videoData.ID,
		CreatedAt:         videoData.CreatedAt,
		UpdatedAt:         time.Now(),
		ThumbnailURL:      &thumbURL,
		VideoURL:          videoData.VideoURL,
		CreateVideoParams: videoData.CreateVideoParams,
	}

	err = cfg.db.UpdateVideo(updateVideoData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Update video error", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updateVideoData)
}
