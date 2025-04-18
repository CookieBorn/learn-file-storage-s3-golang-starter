package main

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	var maxReader io.ReadCloser
	maxReader = http.MaxBytesReader(w, maxReader, maxMemory)

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

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get video data", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unnorthurized", err)
		return
	}

	file, header, err := r.FormFile("video")
	defer file.Close()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't upload video data", err)
		return
	}

	filetype := header.Header.Get("Content-Type")

	Type, _, err := mime.ParseMediaType(filetype)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Mime parse error", err)
		return
	}

	if Type != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Inncorect file type. Expecting mp4", nil)
		return
	}

	vid, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File create error", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer vid.Close()

	_, err = io.Copy(vid, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File copy error", err)
		return
	}

	vidProccessed, err := processVideoForFastStart(vid.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File proccess error", err)
		return
	}
	vidP, err := os.Open(vidProccessed)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File proccess open error", err)
		return
	}
	defer os.Remove(vidProccessed)
	defer vidP.Close()

	_, err = vid.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File seek start error", err)
		return
	}

	key := make([]byte, 32)
	rand.Read(key)
	url := base64.RawURLEncoding.EncodeToString(key)

	aspect, err := getVideoAspectRatio(vid.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Aspect ratio error", err)
		return
	}

	aspectString := "other"

	if aspect == "16:9" {
		aspectString = "landscape"
	} else if aspect == "9:16" {
		aspectString = "portrait"
	}

	fileKey := aspectString + "/" + url + ".mp4"

	putParams := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        vidP,
		ContentType: &Type,
	}

	_, err = cfg.s3client.PutObject(r.Context(), &putParams)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File put error", err)
		return
	}

	videoURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + fileKey

	updateVideoData := database.Video{
		ID:                videoData.ID,
		CreatedAt:         videoData.CreatedAt,
		UpdatedAt:         time.Now(),
		ThumbnailURL:      videoData.ThumbnailURL,
		VideoURL:          &videoURL,
		CreateVideoParams: videoData.CreateVideoParams,
	}

	err = cfg.db.UpdateVideo(updateVideoData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Update video error", err)
		return
	}
}
