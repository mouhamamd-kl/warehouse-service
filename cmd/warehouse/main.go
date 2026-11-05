package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	warehousev1 "github.com/mouhamamd-kl/warehouse-service/gen/warehouse/v1"

	"github.com/mouhamamd-kl/warehouse-service/internal/application/ports/repositories"
	"github.com/mouhamamd-kl/warehouse-service/internal/application/usecases"
	warehousegrpc "github.com/mouhamamd-kl/warehouse-service/internal/infrastructure/grpc"
	"github.com/mouhamamd-kl/warehouse-service/internal/infrastructure/persistence/grpcclient"
	"github.com/mouhamamd-kl/warehouse-service/internal/infrastructure/persistence/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultGRPCTimeoutSeconds = 3

func main() {
	driver := os.Getenv("REPOSITORY_DRIVER")
	if driver == "" {
		driver = "postgres"
	}

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Data source: PostgreSQL (pgx) or a remote warehouse service over
	// gRPC, selected with REPOSITORY_DRIVER.
	var repository repositories.WarehouseRepository

	switch driver {
	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("DATABASE_URL is required")
		}

		db, err := pgxpool.New(ctx, databaseURL)
		if err != nil {
			log.Fatalf("failed to create PostgreSQL pool: %v", err)
		}
		defer db.Close()

		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := db.Ping(pingCtx); err != nil {
			log.Fatalf("failed to connect to PostgreSQL: %v", err)
		}

		repository = postgres.NewRepository(db)

		log.Println("Repository driver: postgres")

	case "grpc":
		target := os.Getenv("WAREHOUSE_GRPC_TARGET")
		if target == "" {
			log.Fatal("WAREHOUSE_GRPC_TARGET is required")
		}

		conn, err := grpc.NewClient(
			target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatalf("failed to create gRPC client: %v", err)
		}
		defer conn.Close()

		repository = grpcclient.NewRepository(
			warehousev1.NewWarehouseServiceClient(conn),
			grpcTimeout(),
		)

		log.Printf("Repository driver: grpc (target=%s)", target)

	default:
		log.Fatalf(
			"unsupported REPOSITORY_DRIVER %q: expected \"postgres\" or \"grpc\"",
			driver,
		)
	}

	// Application.
	warehouseUseCase := usecases.NewWarehouseUseCase(repository)
	reservationUseCase := usecases.NewReservationUseCase(repository)

	// gRPC handlers.
	reservationHandler := warehousegrpc.NewReservationHandler(
		reservationUseCase,
	)

	warehouseHandler := warehousegrpc.NewWarehouseHandler(
		warehouseUseCase,
		reservationHandler,
	)

	// gRPC server.
	server := grpc.NewServer()

	warehousev1.RegisterWarehouseServiceServer(
		server,
		warehouseHandler,
	)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", port, err)
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Warehouse gRPC server listening on :%s", port)
		serverErrors <- server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		log.Println("Shutdown signal received")

	case err := <-serverErrors:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("gRPC server failed: %v", err)
		}
		stop()
	}

	stopped := make(chan struct{})

	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Println("Warehouse service stopped gracefully")

	case <-time.After(10 * time.Second):
		log.Println("Graceful shutdown timed out; forcing shutdown")
		server.Stop()
	}
}

// grpcTimeout reads GRPC_TIMEOUT_SECONDS for per-RPC timeouts in the
// grpcclient adapter.
func grpcTimeout() time.Duration {
	seconds, err := strconv.Atoi(os.Getenv("GRPC_TIMEOUT_SECONDS"))
	if err != nil || seconds < 1 {
		return defaultGRPCTimeoutSeconds * time.Second
	}

	return time.Duration(seconds) * time.Second
}
