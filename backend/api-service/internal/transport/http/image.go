package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
)

const contentDispositionAttachment = `attachment; filename="image.bin"`

type imageUseCase interface {
	Create(ctx context.Context, in usecase.CreateImageInput) (domain.Image, error)
	GetByID(ctx context.Context, imageID uuid.UUID) (domain.Image, error)
	GetByProductID(ctx context.Context, productID uuid.UUID) (domain.Image, error)
	Replace(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) (domain.Image, error)
	Delete(ctx context.Context, imageID uuid.UUID) error
}

type ImageHandler struct {
	imageUseCase imageUseCase
}

func NewImageHandler(imageUseCase imageUseCase) *ImageHandler {
	return &ImageHandler{
		imageUseCase: imageUseCase,
	}
}

type imageMetadataResponse struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
}

func (h *ImageHandler) CreateProductImage(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductID)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageEmptyImageData)
		return
	}

	image, err := h.imageUseCase.Create(r.Context(), usecase.CreateImageInput{
		ProductID:   productID,
		Data:        data,
		ContentType: http.DetectContentType(data),
	})
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			writeError(w, http.StatusNotFound, errCodeProductNotFound, errMessageProductNotFound)
			return
		}
		if errors.Is(err, domain.ErrImageAlreadyExists) {
			writeError(w, http.StatusConflict, errCodeImageAlreadyExists, errMessageImageAlreadyExists)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageCreateImageFailed)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/images/%s", image.ID))
	writeJSON(w, http.StatusCreated, imageMetadataResponse{ID: image.ID, ProductID: image.ProductID})
}

func (h *ImageHandler) GetByProductID(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductID)
		return
	}

	image, err := h.imageUseCase.GetByProductID(r.Context(), productID)
	if err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageImageNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageGetImageFailed)
		return
	}

	writeImageData(w, image)
}

func (h *ImageHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidImageID)
		return
	}

	image, err := h.imageUseCase.GetByID(r.Context(), imageID)
	if err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageImageNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageGetImageFailed)
		return
	}

	writeImageData(w, image)
}

func (h *ImageHandler) Replace(w http.ResponseWriter, r *http.Request) {
	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidImageID)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageEmptyImageData)
		return
	}

	if _, err := h.imageUseCase.Replace(r.Context(), imageID, data, http.DetectContentType(data)); err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageImageNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageReplaceImageFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ImageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidImageID)
		return
	}

	if err := h.imageUseCase.Delete(r.Context(), imageID); err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageImageNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageDeleteImageFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeImageData(w http.ResponseWriter, image domain.Image) {
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Disposition", contentDispositionAttachment)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Data)
}
