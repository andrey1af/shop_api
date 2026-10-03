package http

import (
	jsonv2 "encoding/json/v2"
	"net/http"
)

const (
	errCodeInvalidRequest     = "INVALID_REQUEST"
	errCodeValidationError    = "VALIDATION_ERROR"
	errCodeInternalError      = "INTERNAL_ERROR"
	errCodeNotFound           = "NOT_FOUND"
	errCodeSupplierInUse      = "SUPPLIER_IN_USE"
	errCodeSupplierNotFound   = "SUPPLIER_NOT_FOUND"
	errCodeProductNotFound    = "PRODUCT_NOT_FOUND"
	errCodeImageAlreadyExists = "IMAGE_ALREADY_EXISTS"
	errCodeInsufficientStock  = "INSUFFICIENT_STOCK"
	errCodeUserAlreadyExists  = "USER_ALREADY_EXISTS"
	errCodeInvalidCredentials = "INVALID_CREDENTIALS"
	errCodeOAuthFailed        = "OAUTH_FAILED"
	errCodeInvalidRefresh     = "INVALID_REFRESH_TOKEN"
)

const (
	errMessageInvalidRequestBody  = "invalid request body"
	errMessageInvalidBirthday     = "birthday must be a valid date in YYYY-MM-DD format"
	errMessageInvalidGender       = "gender must be one of: male, female"
	errMessageInvalidLimit        = "limit must be an integer between 1 and 1000"
	errMessageInvalidOffset       = "offset must be a non-negative integer"
	errMessageMissingSearchParams = "client_name and client_surname are required"
	errMessageInvalidClientID     = "clientId must be a valid UUID"
	errMessageInvalidAddress      = "country, city and street are required"
	errMessageCreateClientFailed  = "failed to create client"
	errMessageListClientsFailed   = "failed to list clients"
	errMessageSearchClientsFailed = "failed to find clients"
	errMessageDeleteClientFailed  = "failed to delete client"
	errMessageChangeAddressFailed = "failed to change client address"
	errMessageClientNotFound      = "client not found"

	errMessageInvalidSupplierName         = "name is required"
	errMessageInvalidPhoneNumber          = "phone_number must be in E.164 format, e.g. +79991234567"
	errMessageInvalidSupplierID           = "supplierId must be a valid UUID"
	errMessageCreateSupplierFailed        = "failed to create supplier"
	errMessageListSuppliersFailed         = "failed to list suppliers"
	errMessageGetSupplierFailed           = "failed to get supplier"
	errMessageDeleteSupplierFailed        = "failed to delete supplier"
	errMessageChangeSupplierAddressFailed = "failed to change supplier address"
	errMessageSupplierNotFound            = "supplier not found"
	errMessageSupplierInUse               = "supplier is used by products"

	errMessageInvalidProductName       = "name is required"
	errMessageInvalidProductCategory   = "category is required"
	errMessageInvalidPrice             = "price must be greater than zero"
	errMessageInvalidAvailableStock    = "available_stock must be zero or greater"
	errMessageInvalidRequestSupplierID = "supplier_id must be a valid UUID"
	errMessageInvalidProductID         = "productId must be a valid UUID"
	errMessageCreateProductFailed      = "failed to create product"
	errMessageListProductsFailed       = "failed to list products"
	errMessageGetProductFailed         = "failed to get product"
	errMessageDeleteProductFailed      = "failed to delete product"
	errMessageDecreaseStockFailed      = "failed to decrease product stock"
	errMessageInvalidDecreaseBy        = "decrease_by must be greater than zero"
	errMessageInsufficientStock        = "insufficient stock"
	errMessageProductNotFound          = "product not found"

	errMessageInvalidImageID     = "imageId must be a valid UUID"
	errMessageEmptyImageData     = "image data must not be empty"
	errMessageCreateImageFailed  = "failed to upload image"
	errMessageGetImageFailed     = "failed to get image"
	errMessageReplaceImageFailed = "failed to replace image"
	errMessageDeleteImageFailed  = "failed to delete image"
	errMessageImageNotFound      = "image not found"
	errMessageImageAlreadyExists = "product already has an image"

	errMessageInvalidEmail           = "email must be a valid email address"
	errMessageInvalidUserName        = "name is required"
	errMessageInvalidUserSurname     = "surname is required"
	errMessageInvalidPassword        = "password is required"
	errMessageWeakPassword           = "password must be at least 8 characters"
	errMessageInvalidRegisterRequest = "invalid registration data"
	errMessageUserAlreadyExists      = "user already exists"
	errMessageRegisterFailed         = "failed to register user"

	errMessageInvalidLoginRequest          = "invalid login data"
	errMessageInvalidCredentials           = "invalid email or password"
	errMessageLoginFailed                  = "failed to login"
	errMessageInvalidOldPassword           = "old_password is required"
	errMessageInvalidNewPassword           = "new_password is required"
	errMessageInvalidOldPasswordValue      = "old password is incorrect"
	errMessageInvalidChangePasswordRequest = "invalid change password data"
	errMessageUserNotFound                 = "user not found"
	errMessageChangePasswordFailed         = "failed to change password"
	errMessageResetPasswordFailed          = "failed to reset password"

	errMessageUnsupportedOAuthProvider = "unsupported oauth provider"
	errMessageOAuthStartFailed         = "failed to start oauth login"
	errMessageInvalidOAuthCallback     = "code and state are required"
	errMessageOAuthFailed              = "oauth authorization failed"
	errMessageOAuthUserAlreadyExists   = "user with this email already exists"
	errMessageOAuthCallbackFailed      = "failed to finish oauth login"

	errMessageMissingRefreshToken = "refresh token is required"
	errMessageInvalidRefreshToken = "refresh token is invalid, expired or revoked"
	errMessageRefreshFailed       = "failed to refresh tokens"
	errMessageLogoutFailed        = "failed to logout"
)

func NewRouter(
	requireAuth, requireStreamAuth, rateLimitAuth func(http.Handler) http.Handler,
	health *HealthHandler,
	auth *AuthHandler,
	client *ClientHandler,
	supplier *SupplierHandler,
	product *ProductHandler,
	image *ImageHandler,
	productStream *ProductStreamHandler,
) http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("GET /health/live", health.Liveness)
	router.HandleFunc("GET /health/ready", health.Readiness)

	router.Handle("POST /api/v1/auth/register", rateLimitAuth(http.HandlerFunc(auth.Register)))
	router.Handle("POST /api/v1/auth/login", rateLimitAuth(http.HandlerFunc(auth.Login)))
	router.Handle("POST /api/v1/auth/reset-password", rateLimitAuth(http.HandlerFunc(auth.ResetPassword)))
	router.HandleFunc("POST /api/v1/auth/oauth/{provider}/start", auth.OAuthStart)
	router.HandleFunc("POST /api/v1/auth/oauth/callback", auth.OAuthCallback)

	router.HandleFunc("POST /api/v1/auth/refresh", auth.Refresh)

	router.HandleFunc("POST /api/v1/auth/logout", auth.Logout)

	protected := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(handler)
	}

	router.Handle("POST /api/v1/auth/change-password", protected(auth.ChangePassword))
	router.Handle("POST /api/v1/auth/logout-all", protected(auth.LogoutAll))

	router.Handle("GET /api/v1/clients", protected(client.List))
	router.Handle("POST /api/v1/clients", protected(client.Create))
	router.Handle("GET /api/v1/clients/search", protected(client.FindByNameAndSurname))
	router.Handle("DELETE /api/v1/clients/{clientId}", protected(client.Delete))
	router.Handle("PATCH /api/v1/clients/{clientId}/address", protected(client.ChangeAddress))

	router.Handle("GET /api/v1/suppliers", protected(supplier.List))
	router.Handle("POST /api/v1/suppliers", protected(supplier.Create))
	router.Handle("GET /api/v1/suppliers/{supplierId}", protected(supplier.Get))
	router.Handle("DELETE /api/v1/suppliers/{supplierId}", protected(supplier.Delete))
	router.Handle("PATCH /api/v1/suppliers/{supplierId}/address", protected(supplier.ChangeAddress))

	router.Handle("GET /api/v1/products", protected(product.List))
	router.Handle("POST /api/v1/products", protected(product.Create))
	router.Handle("GET /api/v1/products/{productId}", protected(product.Get))
	router.Handle("DELETE /api/v1/products/{productId}", protected(product.Delete))
	router.Handle("PATCH /api/v1/products/{productId}/stock", protected(product.DecreaseStock))

	router.Handle("GET /api/v1/sse/products", requireStreamAuth(http.HandlerFunc(productStream.Stream)))

	router.Handle("POST /api/v1/products/{productId}/image", protected(image.CreateProductImage))
	router.Handle("GET /api/v1/products/{productId}/image", protected(image.GetByProductID))
	router.Handle("GET /api/v1/images/{imageId}", protected(image.GetByID))
	router.Handle("PUT /api/v1/images/{imageId}", protected(image.Replace))
	router.Handle("DELETE /api/v1/images/{imageId}", protected(image.Delete))

	return router
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = jsonv2.MarshalWrite(w, value)
}
