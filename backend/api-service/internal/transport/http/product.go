package http

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
)

type productUseCase interface {
	Create(ctx context.Context, in usecase.CreateProductInput) (domain.Product, error)
	Delete(ctx context.Context, productID uuid.UUID) error
	Get(ctx context.Context, productID uuid.UUID) (domain.Product, error)
	ListAvailable(ctx context.Context, p domain.ListParams) ([]domain.Product, error)
	DecreaseStock(ctx context.Context, productID uuid.UUID, decreaseBy int64) (domain.Product, error)
}

type ProductHandler struct {
	productUseCase productUseCase
}

func NewProductHandler(productUseCase productUseCase) *ProductHandler {
	return &ProductHandler{
		productUseCase: productUseCase,
	}
}

type createProductRequest struct {
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	Price          float64 `json:"price"`
	AvailableStock int64   `json:"available_stock"`
	SupplierID     string  `json:"supplier_id"`
}

type decreaseStockRequest struct {
	DecreaseBy int64 `json:"decrease_by"`
}

type productResponse struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Category       string     `json:"category"`
	Price          float64    `json:"price"`
	AvailableStock int64      `json:"available_stock"`
	LastUpdateDate time.Time  `json:"last_update_date"`
	SupplierID     uuid.UUID  `json:"supplier_id"`
	ImageID        *uuid.UUID `json:"image_id"`
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	params, message := parseListParams(r.URL.Query())
	if message != "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, message)
		return
	}

	products, err := h.productUseCase.ListAvailable(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageListProductsFailed)
		return
	}

	writeJSON(w, http.StatusOK, toProductResponses(products))
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductName)
		return
	}
	if req.Category == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductCategory)
		return
	}
	if req.Price <= 0 {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidPrice)
		return
	}
	if req.AvailableStock < 0 {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidAvailableStock)
		return
	}
	supplierID, err := uuid.Parse(req.SupplierID)
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidRequestSupplierID)
		return
	}

	product, err := h.productUseCase.Create(r.Context(), usecase.CreateProductInput{
		Name:           req.Name,
		Category:       req.Category,
		Price:          req.Price,
		AvailableStock: req.AvailableStock,
		SupplierID:     supplierID,
	})
	if err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			writeError(w, http.StatusNotFound, errCodeSupplierNotFound, errMessageSupplierNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageCreateProductFailed)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/products/%s", product.ID))
	writeJSON(w, http.StatusCreated, toProductResponse(product))
}

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductID)
		return
	}

	product, err := h.productUseCase.Get(r.Context(), productID)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageProductNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageGetProductFailed)
		return
	}

	writeJSON(w, http.StatusOK, toProductResponse(product))
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductID)
		return
	}

	if err := h.productUseCase.Delete(r.Context(), productID); err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageProductNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageDeleteProductFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) DecreaseStock(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidProductID)
		return
	}

	var req decreaseStockRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if req.DecreaseBy < 1 {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidDecreaseBy)
		return
	}

	product, err := h.productUseCase.DecreaseStock(r.Context(), productID, req.DecreaseBy)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageProductNotFound)
			return
		}
		if errors.Is(err, domain.ErrInsufficientStock) {
			writeError(w, http.StatusBadRequest, errCodeInsufficientStock, errMessageInsufficientStock)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageDecreaseStockFailed)
		return
	}

	writeJSON(w, http.StatusOK, toProductResponse(product))
}

func toProductResponses(products []domain.Product) []productResponse {
	responses := make([]productResponse, 0, len(products))
	for _, product := range products {
		responses = append(responses, toProductResponse(product))
	}
	return responses
}

func toProductResponse(product domain.Product) productResponse {
	return productResponse{
		ID:             product.ID,
		Name:           product.Name,
		Category:       product.Category,
		Price:          product.Price,
		AvailableStock: product.AvailableStock,
		LastUpdateDate: product.LastUpdateDate,
		SupplierID:     product.SupplierID,
		ImageID:        product.ImageID,
	}
}

func parseListParams(query url.Values) (domain.ListParams, string) {
	var params domain.ListParams

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 1000 {
			return domain.ListParams{}, errMessageInvalidLimit
		}
		params.Limit = &limit
	}

	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return domain.ListParams{}, errMessageInvalidOffset
		}
		params.Offset = &offset
	}

	return params, ""
}
