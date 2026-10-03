package http

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
	"github.com/andrey1af/shop-api/backend/api-service/pkg/validation"
)

type supplierUseCase interface {
	Create(ctx context.Context, in usecase.CreateSupplierInput) (domain.Supplier, error)
	Delete(ctx context.Context, supplierID uuid.UUID) error
	Get(ctx context.Context, supplierID uuid.UUID) (domain.Supplier, error)
	List(ctx context.Context) ([]domain.Supplier, error)
	ChangeAddress(ctx context.Context, supplierID uuid.UUID, in usecase.CreateAddressInput) (domain.Supplier, error)
}

type SupplierHandler struct {
	supplierUseCase supplierUseCase
}

func NewSupplierHandler(supplierUseCase supplierUseCase) *SupplierHandler {
	return &SupplierHandler{
		supplierUseCase: supplierUseCase,
	}
}

type createSupplierRequest struct {
	Name        string         `json:"name"`
	Address     addressRequest `json:"address"`
	PhoneNumber string         `json:"phone_number"`
}

type supplierResponse struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Address     addressResponse `json:"address"`
	PhoneNumber string          `json:"phone_number"`
}

func (h *SupplierHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSupplierRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidSupplierName)
		return
	}

	if req.Address.Country == "" || req.Address.City == "" || req.Address.Street == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidAddress)
		return
	}

	if !validation.IsValidPhoneNumber(req.PhoneNumber) {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidPhoneNumber)
		return
	}

	supplier, err := h.supplierUseCase.Create(r.Context(), usecase.CreateSupplierInput{
		Name:        req.Name,
		PhoneNumber: req.PhoneNumber,
		Address: usecase.CreateAddressInput{
			Country: req.Address.Country,
			City:    req.Address.City,
			Street:  req.Address.Street,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageCreateSupplierFailed)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/suppliers/%s", supplier.ID))
	writeJSON(w, http.StatusCreated, toSupplierResponse(supplier))
}

func (h *SupplierHandler) List(w http.ResponseWriter, r *http.Request) {
	suppliers, err := h.supplierUseCase.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageListSuppliersFailed)
		return
	}

	writeJSON(w, http.StatusOK, toSupplierResponses(suppliers))
}

func (h *SupplierHandler) Get(w http.ResponseWriter, r *http.Request) {
	supplierID, err := uuid.Parse(r.PathValue("supplierId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidSupplierID)
		return
	}

	supplier, err := h.supplierUseCase.Get(r.Context(), supplierID)
	if err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageSupplierNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageGetSupplierFailed)
		return
	}

	writeJSON(w, http.StatusOK, toSupplierResponse(supplier))
}

func (h *SupplierHandler) Delete(w http.ResponseWriter, r *http.Request) {
	supplierID, err := uuid.Parse(r.PathValue("supplierId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidSupplierID)
		return
	}

	if err := h.supplierUseCase.Delete(r.Context(), supplierID); err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageSupplierNotFound)
			return
		}
		if errors.Is(err, domain.ErrSupplierInUse) {
			writeError(w, http.StatusConflict, errCodeSupplierInUse, errMessageSupplierInUse)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageDeleteSupplierFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SupplierHandler) ChangeAddress(w http.ResponseWriter, r *http.Request) {
	supplierID, err := uuid.Parse(r.PathValue("supplierId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidSupplierID)
		return
	}

	var req addressRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if req.Country == "" || req.City == "" || req.Street == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidAddress)
		return
	}

	supplier, err := h.supplierUseCase.ChangeAddress(r.Context(), supplierID, usecase.CreateAddressInput{
		Country: req.Country,
		City:    req.City,
		Street:  req.Street,
	})
	if err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageSupplierNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageChangeSupplierAddressFailed)
		return
	}

	writeJSON(w, http.StatusOK, toSupplierResponse(supplier))
}

func toSupplierResponses(suppliers []domain.Supplier) []supplierResponse {
	responses := make([]supplierResponse, 0, len(suppliers))
	for _, supplier := range suppliers {
		responses = append(responses, toSupplierResponse(supplier))
	}
	return responses
}

func toSupplierResponse(supplier domain.Supplier) supplierResponse {
	return supplierResponse{
		ID:          supplier.ID,
		Name:        supplier.Name,
		PhoneNumber: supplier.PhoneNumber,
		Address: addressResponse{
			ID:      supplier.Address.ID,
			Country: supplier.Address.Country,
			City:    supplier.Address.City,
			Street:  supplier.Address.Street,
		},
	}
}
