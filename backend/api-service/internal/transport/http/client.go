package http

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
)

const birthdayLayout = "2006-01-02"

type clientUseCase interface {
	Create(ctx context.Context, in usecase.CreateClientInput) (domain.Client, error)
	Delete(ctx context.Context, clientID uuid.UUID) error
	FindByNameAndSurname(ctx context.Context, name, surname string) ([]domain.Client, error)
	List(ctx context.Context, p domain.ListParams) ([]domain.Client, error)
	ChangeAddress(ctx context.Context, clientID uuid.UUID, in usecase.CreateAddressInput) (domain.Client, error)
}

type ClientHandler struct {
	clientUseCase clientUseCase
}

func NewClientHandler(clientUseCase clientUseCase) *ClientHandler {
	return &ClientHandler{
		clientUseCase: clientUseCase,
	}
}

type addressRequest struct {
	Country string `json:"country"`
	City    string `json:"city"`
	Street  string `json:"street"`
}

type createClientRequest struct {
	ClientName    string         `json:"client_name"`
	ClientSurname string         `json:"client_surname"`
	Birthday      string         `json:"birthday"`
	Gender        string         `json:"gender"`
	Address       addressRequest `json:"address"`
}

type addressResponse struct {
	ID      uuid.UUID `json:"id"`
	Country string    `json:"country"`
	City    string    `json:"city"`
	Street  string    `json:"street"`
}

type clientResponse struct {
	ID               uuid.UUID       `json:"id"`
	ClientName       string          `json:"client_name"`
	ClientSurname    string          `json:"client_surname"`
	Birthday         string          `json:"birthday"`
	Gender           string          `json:"gender"`
	RegistrationDate time.Time       `json:"registration_date"`
	Address          addressResponse `json:"address"`
}

func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createClientRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	birthday, err := time.Parse(birthdayLayout, req.Birthday)
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidBirthday)
		return
	}

	gender, ok := genderFromRequest(req.Gender)
	if !ok {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidGender)
		return
	}

	client, err := h.clientUseCase.Create(r.Context(), usecase.CreateClientInput{
		Name:     req.ClientName,
		Surname:  req.ClientSurname,
		Birthday: birthday,
		Gender:   gender,
		Address: usecase.CreateAddressInput{
			Country: req.Address.Country,
			City:    req.Address.City,
			Street:  req.Address.Street,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageCreateClientFailed)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/clients/%s", client.ID))
	writeJSON(w, http.StatusCreated, toClientResponse(client))
}

func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	var params domain.ListParams

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 1000 {
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidLimit)
			return
		}
		params.Limit = &limit
	}

	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidOffset)
			return
		}
		params.Offset = &offset
	}

	clients, err := h.clientUseCase.List(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageListClientsFailed)
		return
	}

	writeJSON(w, http.StatusOK, toClientResponses(clients))
}

func (h *ClientHandler) FindByNameAndSurname(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	name := query.Get("client_name")
	surname := query.Get("client_surname")

	if name == "" || surname == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageMissingSearchParams)
		return
	}

	clients, err := h.clientUseCase.FindByNameAndSurname(r.Context(), name, surname)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageSearchClientsFailed)
		return
	}

	writeJSON(w, http.StatusOK, toClientResponses(clients))
}

func (h *ClientHandler) Delete(w http.ResponseWriter, r *http.Request) {
	clientID, err := uuid.Parse(r.PathValue("clientId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidClientID)
		return
	}

	if err := h.clientUseCase.Delete(r.Context(), clientID); err != nil {
		if errors.Is(err, domain.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageClientNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageDeleteClientFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ClientHandler) ChangeAddress(w http.ResponseWriter, r *http.Request) {
	clientID, err := uuid.Parse(r.PathValue("clientId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidClientID)
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

	client, err := h.clientUseCase.ChangeAddress(r.Context(), clientID, usecase.CreateAddressInput{
		Country: req.Country,
		City:    req.City,
		Street:  req.Street,
	})
	if err != nil {
		if errors.Is(err, domain.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageClientNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageChangeAddressFailed)
		return
	}

	writeJSON(w, http.StatusOK, toClientResponse(client))
}

func genderFromRequest(value string) (domain.GenderType, bool) {
	switch value {
	case "male":
		return domain.GenderMale, true
	case "female":
		return domain.GenderFemale, true
	default:
		return "", false
	}
}

func genderToResponse(gender domain.GenderType) string {
	switch gender {
	case domain.GenderMale:
		return "male"
	case domain.GenderFemale:
		return "female"
	default:
		return ""
	}
}

func toClientResponses(clients []domain.Client) []clientResponse {
	responses := make([]clientResponse, 0, len(clients))
	for _, client := range clients {
		responses = append(responses, toClientResponse(client))
	}
	return responses
}

func toClientResponse(client domain.Client) clientResponse {
	return clientResponse{
		ID:               client.ID,
		ClientName:       client.Name,
		ClientSurname:    client.Surname,
		Birthday:         client.Birthday.Format(birthdayLayout),
		Gender:           genderToResponse(client.Gender),
		RegistrationDate: client.RegistrationTime,
		Address: addressResponse{
			ID:      client.Address.ID,
			Country: client.Address.Country,
			City:    client.Address.City,
			Street:  client.Address.Street,
		},
	}
}
