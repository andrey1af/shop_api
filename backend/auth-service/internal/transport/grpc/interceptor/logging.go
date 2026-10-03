package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func Logging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		log = log.With(
			slog.String("method", info.FullMethod),
			slog.String("remote_addr", remoteAddr(ctx)),
		)

		log.InfoContext(ctx, "request started")

		resp, err := handler(ctx, req)

		code := status.Code(err)
		attrs := []any{
			slog.String("code", code.String()),
			slog.Duration("duration", time.Since(start)),
		}

		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}

		switch code {
		case codes.OK:
			log.InfoContext(ctx, "request completed", attrs...)
		case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable:
			log.ErrorContext(ctx, "request failed", attrs...)
		default:
			log.WarnContext(ctx, "request failed", attrs...)
		}

		return resp, err
	}
}

func remoteAddr(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	return p.Addr.String()
}
