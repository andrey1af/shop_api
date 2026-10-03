package grpc

import (
	"context"
	"errors"
	"uuid"

	"github.com/andrey1af/shop-api/backend/gen/imagev1"
	apperrors "github.com/andrey1af/shop-api/backend/image-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/image-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type imageUseCase interface {
	Create(ctx context.Context, in usecase.CreateImageInput) error
	Get(ctx context.Context, id uuid.UUID) (usecase.ImageOutput, error)
	Replace(ctx context.Context, in usecase.ReplaceImageInput) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Server struct {
	imagev1.UnimplementedImageServiceServer
	images imageUseCase
}

func RegisterServer(gRPC *grpc.Server, images imageUseCase) {
	imagev1.RegisterImageServiceServer(gRPC, &Server{images: images})
}

func (s *Server) CreateImage(
	ctx context.Context,
	req *imagev1.CreateImageRequest,
) (*imagev1.CreateImageResponse, error) {
	id, err := parseImageID(req.GetId())
	if err != nil {
		return nil, err
	}
	if len(req.GetData()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "data is required")
	}

	if err := s.images.Create(ctx, usecase.CreateImageInput{
		ID:          id,
		Data:        req.GetData(),
		ContentType: req.GetContentType(),
	}); err != nil {
		return nil, toStatusError(err)
	}

	return &imagev1.CreateImageResponse{}, nil
}

func (s *Server) GetImage(ctx context.Context, req *imagev1.GetImageRequest) (*imagev1.GetImageResponse, error) {
	id, err := parseImageID(req.GetId())
	if err != nil {
		return nil, err
	}

	image, err := s.images.Get(ctx, id)
	if err != nil {
		return nil, toStatusError(err)
	}

	return &imagev1.GetImageResponse{
		Id:          image.ID.String(),
		Data:        image.Data,
		ContentType: image.ContentType,
	}, nil
}

func (s *Server) ReplaceImage(
	ctx context.Context,
	req *imagev1.ReplaceImageRequest,
) (*imagev1.ReplaceImageResponse, error) {
	id, err := parseImageID(req.GetId())
	if err != nil {
		return nil, err
	}
	if len(req.GetData()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "data is required")
	}

	if err := s.images.Replace(ctx, usecase.ReplaceImageInput{
		ID:          id,
		Data:        req.GetData(),
		ContentType: req.GetContentType(),
	}); err != nil {
		return nil, toStatusError(err)
	}

	return &imagev1.ReplaceImageResponse{}, nil
}

func (s *Server) DeleteImage(
	ctx context.Context,
	req *imagev1.DeleteImageRequest,
) (*imagev1.DeleteImageResponse, error) {
	id, err := parseImageID(req.GetId())
	if err != nil {
		return nil, err
	}

	if err := s.images.Delete(ctx, id); err != nil {
		return nil, toStatusError(err)
	}

	return &imagev1.DeleteImageResponse{}, nil
}

func parseImageID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "id must be a valid UUID")
	}

	return id, nil
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, apperrors.ErrImageNotFound):
		return status.Error(codes.NotFound, "image not found")
	case errors.Is(err, apperrors.ErrImageAlreadyExists):
		return status.Error(codes.AlreadyExists, "image already exists")
	case errors.Is(err, apperrors.ErrEmptyImageData):
		return status.Error(codes.InvalidArgument, "data is required")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
