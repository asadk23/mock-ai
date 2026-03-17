package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/asadk23/mock-ai/internal/api"
	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/model"
)

// mockAudioContent is a minimal valid byte sequence returned by the speech endpoint.
// Real audio files would have proper headers; this is sufficient for testing.
var _mockAudioContent = []byte("mock-audio-content")

// defaultAudioFormat is the default format when none is specified.
const defaultAudioFormat = "mp3"

// AudioHandler handles Audio API endpoints.
type AudioHandler struct {
	fixtures *fixture.Fixtures
	logger   *zerolog.Logger
}

// NewAudioHandler creates a new AudioHandler with the given dependencies.
func NewAudioHandler(f *fixture.Fixtures, logger *zerolog.Logger) *AudioHandler {
	return &AudioHandler{
		fixtures: f,
		logger:   logger,
	}
}

// Routes returns a chi.Router with all audio routes registered.
func (h *AudioHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/speech", h.Speech)
	r.Post("/transcriptions", h.Transcription)
	r.Post("/translations", h.Translation)

	return r
}

// Speech handles POST /v1/audio/speech.
//
// It validates the request body and returns mock audio bytes with the
// appropriate Content-Type for the requested format (default: mp3).
func (h *AudioHandler) Speech(w http.ResponseWriter, r *http.Request) {
	var req model.AudioSpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest,
			"We could not parse the JSON body of your request.",
			model.ErrTypeInvalidRequest, model.ErrCodeInvalidRequest)
		return
	}

	if err := validateSpeechRequest(&req); err != nil {
		api.WriteErrorWithParam(w, http.StatusBadRequest,
			err.Error(), model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, err.param)
		return
	}

	format := defaultAudioFormat
	if req.ResponseFormat != nil && *req.ResponseFormat != "" {
		if !model.IsValidAudioFormat(*req.ResponseFormat) {
			api.WriteErrorWithParam(w, http.StatusBadRequest,
				"Invalid response format '"+*req.ResponseFormat+"'.",
				model.ErrTypeInvalidRequest, model.ErrCodeInvalidValue, "response_format")
			return
		}
		format = *req.ResponseFormat
	}

	contentType := model.AudioContentType(format)
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(_mockAudioContent); err != nil {
		h.logger.Error().Err(err).Msg("failed to write audio response")
	}
}

// Transcription handles POST /v1/audio/transcriptions.
//
// It expects a multipart form with a "file" field and a "model" field.
// Returns a mock transcription using fixture data.
func (h *AudioHandler) Transcription(w http.ResponseWriter, r *http.Request) {
	if err := h.validateMultipartAudio(r); err != nil {
		api.WriteErrorWithParam(w, http.StatusBadRequest,
			err.Error(), model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, err.param)
		return
	}

	api.WriteJSON(w, http.StatusOK, model.AudioTranscriptionResponse{
		Text: h.fixtures.AudioTranscription.Text,
	})
}

// Translation handles POST /v1/audio/translations.
//
// It expects a multipart form with a "file" field and a "model" field.
// Returns a mock translation using fixture data (same text as transcription
// since this is a mock server).
func (h *AudioHandler) Translation(w http.ResponseWriter, r *http.Request) {
	if err := h.validateMultipartAudio(r); err != nil {
		api.WriteErrorWithParam(w, http.StatusBadRequest,
			err.Error(), model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, err.param)
		return
	}

	api.WriteJSON(w, http.StatusOK, model.AudioTranslationResponse{
		Text: h.fixtures.AudioTranscription.Text,
	})
}

// multipartMaxMemory is the max memory for multipart form parsing (10 MB).
const multipartMaxMemory = 10 << 20

// validateMultipartAudio parses a multipart form and checks for required fields.
func (h *AudioHandler) validateMultipartAudio(r *http.Request) *validationError {
	if err := r.ParseMultipartForm(multipartMaxMemory); err != nil {
		return &validationError{
			message: "Request must be a multipart form.",
			param:   "file",
		}
	}

	if r.FormValue("model") == "" {
		return &validationError{
			message: "you must provide a model parameter",
			param:   "model",
		}
	}

	_, _, err := r.FormFile("file")
	if err != nil {
		return &validationError{
			message: "you must provide a file parameter",
			param:   "file",
		}
	}

	return nil
}

// validateSpeechRequest checks required fields on the speech request.
func validateSpeechRequest(req *model.AudioSpeechRequest) *validationError {
	if req.Model == "" {
		return &validationError{
			message: "you must provide a model parameter",
			param:   "model",
		}
	}

	if req.Input == "" {
		return &validationError{
			message: "you must provide an input parameter",
			param:   "input",
		}
	}

	if req.Voice == "" {
		return &validationError{
			message: "you must provide a voice parameter",
			param:   "voice",
		}
	}

	return nil
}
