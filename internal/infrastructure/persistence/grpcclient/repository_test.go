package grpcclient

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	warehousev1 "github.com/mouhamamd-kl/warehouse-service/gen/warehouse/v1"
	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeWarehouseServer answers RPCs with canned responses/errors and
// captures the last requests, served over an in-memory bufconn listener.
type fakeWarehouseServer struct {
	warehousev1.UnimplementedWarehouseServiceServer

	inventory     *warehousev1.InventoryResponse
	inventoryList *warehousev1.ListInventoryResponse
	movements     *warehousev1.GetMovementsResponse
	reservation   *warehousev1.ReservationResponse

	lastInventoryList *warehousev1.ListInventoryRequest
	lastAdjustStock   *warehousev1.AdjustStockRequest
	lastReserveStock  *warehousev1.ReserveStockRequest

	errCreateInventory    error
	errGetInventory       error
	errGetAllInventory    error
	errGetLowStock        error
	errAdjustStock        error
	errGetMovements       error
	errReserveStock       error
	errGetReservation     error
	errReleaseReservation error
	errConfirmReservation error
}

func (s *fakeWarehouseServer) CreateInventory(
	context.Context,
	*warehousev1.CreateInventoryRequest,
) (*warehousev1.InventoryResponse, error) {
	if s.errCreateInventory != nil {
		return nil, s.errCreateInventory
	}

	return s.inventory, nil
}

func (s *fakeWarehouseServer) GetInventory(
	context.Context,
	*warehousev1.GetInventoryRequest,
) (*warehousev1.InventoryResponse, error) {
	if s.errGetInventory != nil {
		return nil, s.errGetInventory
	}

	return s.inventory, nil
}

func (s *fakeWarehouseServer) GetAllInventory(
	ctx context.Context,
	req *warehousev1.ListInventoryRequest,
) (*warehousev1.ListInventoryResponse, error) {
	if s.errGetAllInventory != nil {
		return nil, s.errGetAllInventory
	}

	s.lastInventoryList = req

	return s.inventoryList, nil
}

func (s *fakeWarehouseServer) GetLowStock(
	ctx context.Context,
	req *warehousev1.ListInventoryRequest,
) (*warehousev1.ListInventoryResponse, error) {
	if s.errGetLowStock != nil {
		return nil, s.errGetLowStock
	}

	s.lastInventoryList = req

	return s.inventoryList, nil
}

func (s *fakeWarehouseServer) AdjustStock(
	ctx context.Context,
	req *warehousev1.AdjustStockRequest,
) (*warehousev1.InventoryResponse, error) {
	if s.errAdjustStock != nil {
		return nil, s.errAdjustStock
	}

	s.lastAdjustStock = req

	return s.inventory, nil
}

func (s *fakeWarehouseServer) GetMovements(
	context.Context,
	*warehousev1.GetMovementsRequest,
) (*warehousev1.GetMovementsResponse, error) {
	if s.errGetMovements != nil {
		return nil, s.errGetMovements
	}

	return s.movements, nil
}

func (s *fakeWarehouseServer) ReserveStock(
	ctx context.Context,
	req *warehousev1.ReserveStockRequest,
) (*warehousev1.ReservationResponse, error) {
	if s.errReserveStock != nil {
		return nil, s.errReserveStock
	}

	s.lastReserveStock = req

	return s.reservation, nil
}

func (s *fakeWarehouseServer) GetReservation(
	context.Context,
	*warehousev1.GetReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	if s.errGetReservation != nil {
		return nil, s.errGetReservation
	}

	return s.reservation, nil
}

func (s *fakeWarehouseServer) ReleaseReservation(
	context.Context,
	*warehousev1.ReleaseReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	if s.errReleaseReservation != nil {
		return nil, s.errReleaseReservation
	}

	return s.reservation, nil
}

func (s *fakeWarehouseServer) ConfirmReservation(
	context.Context,
	*warehousev1.ConfirmReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	if s.errConfirmReservation != nil {
		return nil, s.errConfirmReservation
	}

	return s.reservation, nil
}

func newTestRepository(t *testing.T, fake *fakeWarehouseServer) *Repository {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer()
	warehousev1.RegisterWarehouseServiceServer(server, fake)

	t.Cleanup(server.Stop)

	go server.Serve(listener)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(
			func(ctx context.Context, _ string) (net.Conn, error) {
				return listener.DialContext(ctx)
			},
		),
	)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close test connection: %v", err)
		}
	})

	return NewRepository(
		warehousev1.NewWarehouseServiceClient(conn),
		5*time.Second,
	)
}

func rfc3339(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("failed to parse test timestamp: %v", err)
	}

	return parsed
}

func sampleInventoryResponse() *warehousev1.InventoryResponse {
	return &warehousev1.InventoryResponse{
		Id:                1,
		ProductId:         7,
		QuantityOnHand:    30,
		QuantityReserved:  5,
		AvailableQuantity: 25,
		LowStockThreshold: 5,
		IsLowStock:        false,
		CreatedAt:         "2026-11-04T09:30:00Z",
		UpdatedAt:         "2026-11-04T10:45:00Z",
	}
}

func sampleReservationResponse() *warehousev1.ReservationResponse {
	return &warehousev1.ReservationResponse{
		Id:      "res-1",
		OrderId: "order-9",
		Status:  "active",
		Items: []*warehousev1.ReservationItem{
			{ProductId: 3, Quantity: 2},
			{ProductId: 8, Quantity: 1},
		},
		ExpiresAt: "2026-11-05T09:30:00Z",
		CreatedAt: "2026-11-04T09:30:00Z",
	}
}

func TestRepositoryCreateInventory(t *testing.T) {
	t.Run("maps the response to the entity", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			inventory: sampleInventoryResponse(),
		}

		repository := newTestRepository(t, fake)

		inventory, err := repository.CreateInventory(t.Context(), &entities.Inventory{
			ProductID:         7,
			QuantityOnHand:    30,
			LowStockThreshold: 5,
		})
		if err != nil {
			t.Fatalf("CreateInventory returned error: %v", err)
		}

		if inventory.ID != 1 || inventory.ProductID != 7 {
			t.Fatalf("unexpected identity fields: %+v", inventory)
		}

		if inventory.CreatedAt.UTC() != rfc3339(t, "2026-11-04T09:30:00Z") {
			t.Fatalf("unexpected created_at: %v", inventory.CreatedAt)
		}

		if inventory.UpdatedAt.UTC() != rfc3339(t, "2026-11-04T10:45:00Z") {
			t.Fatalf("unexpected updated_at: %v", inventory.UpdatedAt)
		}
	})

	t.Run("decodes AlreadyExists", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errCreateInventory: status.Error(
				codes.AlreadyExists, "inventory already exists"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.CreateInventory(t.Context(), &entities.Inventory{
			ProductID:         7,
			QuantityOnHand:    0,
			LowStockThreshold: 5,
		})
		if !errors.Is(err, domainerrors.ErrInventoryAlreadyExists) {
			t.Fatalf("expected ErrInventoryAlreadyExists, got: %v", err)
		}
	})
}

func TestRepositoryFindByProductID(t *testing.T) {
	t.Run("returns ErrInventoryNotFound", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errGetInventory: status.Error(codes.NotFound, "inventory not found"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.FindByProductID(t.Context(), 7)
		if !errors.Is(err, domainerrors.ErrInventoryNotFound) {
			t.Fatalf("expected ErrInventoryNotFound, got: %v", err)
		}
	})
}

func TestRepositoryFindAll(t *testing.T) {
	fake := &fakeWarehouseServer{
		inventoryList: &warehousev1.ListInventoryResponse{
			Items: []*warehousev1.InventoryResponse{
				sampleInventoryResponse(),
			},
			Total: 42,
			Page:  3,
			Limit: 10,
		},
	}

	repository := newTestRepository(t, fake)

	items, total, err := repository.FindAll(t.Context(), 3, 10)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}

	if total != 42 {
		t.Fatalf("expected total 42, got %d", total)
	}

	if len(items) != 1 || items[0].ProductID != 7 {
		t.Fatalf("unexpected items: %+v", items)
	}

	list := fake.lastInventoryList

	if list.GetPage() != 3 || list.GetLimit() != 10 {
		t.Fatalf("pagination not passed through: %+v", list)
	}
}

func TestRepositoryFindLowStock(t *testing.T) {
	fake := &fakeWarehouseServer{
		inventoryList: &warehousev1.ListInventoryResponse{
			Items: []*warehousev1.InventoryResponse{
				sampleInventoryResponse(),
			},
			Total: 1,
			Page:  1,
			Limit: 20,
		},
	}

	repository := newTestRepository(t, fake)

	items, total, err := repository.FindLowStock(t.Context(), 1, 20)
	if err != nil {
		t.Fatalf("FindLowStock returned error: %v", err)
	}

	if total != 1 || len(items) != 1 {
		t.Fatalf("unexpected result: %d item(s), total %d", len(items), total)
	}

	if items[0].AvailableQuantity() != 25 {
		t.Fatalf("unexpected available quantity: %d", items[0].AvailableQuantity())
	}
}

func TestRepositoryAdjustStock(t *testing.T) {
	t.Run("passes the request through and maps the response", func(t *testing.T) {
		fake := &fakeWarehouseServer{inventory: sampleInventoryResponse()}

		repository := newTestRepository(t, fake)

		inventory, err := repository.AdjustStock(
			t.Context(),
			7,
			10,
			"add",
			"Restock",
			"ref-1",
			"idem-1",
		)
		if err != nil {
			t.Fatalf("AdjustStock returned error: %v", err)
		}

		if inventory.ID != 1 {
			t.Fatalf("unexpected inventory: %+v", inventory)
		}

		request := fake.lastAdjustStock

		if request.GetProductId() != 7 ||
			request.GetQuantity() != 10 ||
			request.GetOperation() != "add" ||
			request.GetReason() != "Restock" ||
			request.GetReferenceId() != "ref-1" ||
			request.GetIdempotencyKey() != "idem-1" {
			t.Fatalf("unexpected request: %+v", request)
		}
	})

	t.Run("decodes FailedPrecondition into ErrInsufficientStock", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errAdjustStock: status.Error(
				codes.FailedPrecondition, "insufficient available stock"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.AdjustStock(t.Context(), 7, 10, "remove", "sale", "", "idem-1")
		if !errors.Is(err, domainerrors.ErrInsufficientStock) {
			t.Fatalf("expected ErrInsufficientStock, got: %v", err)
		}
	})

	t.Run("decodes AlreadyExists into ErrIdempotencyConflict", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errAdjustStock: status.Error(
				codes.AlreadyExists, "idempotency key was used with different data"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.AdjustStock(t.Context(), 7, 10, "add", "sale", "", "idem-1")
		if !errors.Is(err, domainerrors.ErrIdempotencyConflict) {
			t.Fatalf("expected ErrIdempotencyConflict, got: %v", err)
		}
	})
}

func TestRepositoryGetMovements(t *testing.T) {
	fake := &fakeWarehouseServer{
		movements: &warehousev1.GetMovementsResponse{
			Items: []*warehousev1.MovementResponse{
				{
					Id:             5,
					InventoryId:    1,
					ProductId:      7,
					Type:           "stock_added",
					Quantity:       30,
					QuantityBefore: 0,
					QuantityAfter:  30,
					Reason:         "Initial inventory",
					ReferenceId:    "ref-1",
					CreatedAt:      "2026-11-04T09:31:00Z",
				},
			},
			Total: 1,
		},
	}

	repository := newTestRepository(t, fake)

	movements, total, err := repository.GetMovements(t.Context(), 7, 1, 20)
	if err != nil {
		t.Fatalf("GetMovements returned error: %v", err)
	}

	if total != 1 || len(movements) != 1 {
		t.Fatalf("unexpected result: %d item(s), total %d", len(movements), total)
	}

	movement := movements[0]

	if movement.Type != entities.MovementStockAdded ||
		movement.InventoryID != 1 ||
		movement.Quantity != 30 ||
		movement.ReferenceID != "ref-1" {
		t.Fatalf("unexpected movement: %+v", movement)
	}

	if movement.CreatedAt.UTC() != rfc3339(t, "2026-11-04T09:31:00Z") {
		t.Fatalf("unexpected created_at: %v", movement.CreatedAt)
	}

	t.Run("decodes NotFound into ErrInventoryNotFound", func(t *testing.T) {
		notFound := &fakeWarehouseServer{
			errGetMovements: status.Error(codes.NotFound, "inventory not found"),
		}

		repository := newTestRepository(t, notFound)

		_, _, err := repository.GetMovements(t.Context(), 7, 1, 20)
		if !errors.Is(err, domainerrors.ErrInventoryNotFound) {
			t.Fatalf("expected ErrInventoryNotFound, got: %v", err)
		}
	})
}

func TestRepositoryReserveStock(t *testing.T) {
	t.Run("builds the request and maps the response", func(t *testing.T) {
		fake := &fakeWarehouseServer{reservation: sampleReservationResponse()}

		repository := newTestRepository(t, fake)

		expiresAt := time.Now().UTC().Add(90 * time.Second)

		reservation, err := repository.ReserveStock(t.Context(), &entities.Reservation{
			OrderID:        "order-9",
			IdempotencyKey: "idem-9",
			Status:         entities.ReservationActive,
			Items: []entities.ReservationItem{
				{ProductID: 3, Quantity: 2},
				{ProductID: 8, Quantity: 1},
			},
			ExpiresAt: &expiresAt,
		})
		if err != nil {
			t.Fatalf("ReserveStock returned error: %v", err)
		}

		request := fake.lastReserveStock

		if request.GetOrderId() != "order-9" || request.GetIdempotencyKey() != "idem-9" {
			t.Fatalf("unexpected request identifiers: %+v", request)
		}

		if len(request.GetItems()) != 2 ||
			request.GetItems()[0].GetProductId() != 3 ||
			request.GetItems()[1].GetQuantity() != 1 {
			t.Fatalf("unexpected items: %+v", request.GetItems())
		}

		if request.GetExpiresInSeconds() < 80 || request.GetExpiresInSeconds() > 90 {
			t.Fatalf("unexpected expires_in_seconds: %d", request.GetExpiresInSeconds())
		}

		if reservation.ID != "res-1" ||
			reservation.Status != entities.ReservationActive ||
			len(reservation.Items) != 2 ||
			reservation.ExpiresAt == nil {
			t.Fatalf("unexpected reservation: %+v", reservation)
		}
	})

	t.Run("omits expires_in_seconds when there is no expiry", func(t *testing.T) {
		fake := &fakeWarehouseServer{reservation: sampleReservationResponse()}

		repository := newTestRepository(t, fake)

		_, err := repository.ReserveStock(t.Context(), &entities.Reservation{
			OrderID:        "order-9",
			IdempotencyKey: "idem-9",
			Items: []entities.ReservationItem{
				{ProductID: 3, Quantity: 2},
			},
		})
		if err != nil {
			t.Fatalf("ReserveStock returned error: %v", err)
		}

		if fake.lastReserveStock.GetExpiresInSeconds() != 0 {
			t.Fatalf(
				"expected expiry to be omitted, got %d",
				fake.lastReserveStock.GetExpiresInSeconds(),
			)
		}
	})

	t.Run("decodes AlreadyExists into ErrIdempotencyConflict", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errReserveStock: status.Error(
				codes.AlreadyExists, "idempotency key was used with different data"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.ReserveStock(t.Context(), &entities.Reservation{
			OrderID:        "order-9",
			IdempotencyKey: "idem-9",
		})
		if !errors.Is(err, domainerrors.ErrIdempotencyConflict) {
			t.Fatalf("expected ErrIdempotencyConflict, got: %v", err)
		}
	})

	t.Run("decodes FailedPrecondition into ErrInsufficientStock", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errReserveStock: status.Error(
				codes.FailedPrecondition, "insufficient available stock"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.ReserveStock(t.Context(), &entities.Reservation{
			OrderID:        "order-9",
			IdempotencyKey: "idem-9",
		})
		if !errors.Is(err, domainerrors.ErrInsufficientStock) {
			t.Fatalf("expected ErrInsufficientStock, got: %v", err)
		}
	})
}

func TestRepositoryFindReservation(t *testing.T) {
	t.Run("returns ErrReservationNotFound", func(t *testing.T) {
		fake := &fakeWarehouseServer{
			errGetReservation: status.Error(codes.NotFound, "reservation not found"),
		}

		repository := newTestRepository(t, fake)

		_, err := repository.FindReservation(t.Context(), "res-1")
		if !errors.Is(err, domainerrors.ErrReservationNotFound) {
			t.Fatalf("expected ErrReservationNotFound, got: %v", err)
		}
	})

	t.Run("maps the response", func(t *testing.T) {
		fake := &fakeWarehouseServer{reservation: sampleReservationResponse()}

		repository := newTestRepository(t, fake)

		reservation, err := repository.FindReservation(t.Context(), "res-1")
		if err != nil {
			t.Fatalf("FindReservation returned error: %v", err)
		}

		if reservation.OrderID != "order-9" ||
			len(reservation.Items) != 2 ||
			reservation.CreatedAt.UTC() != rfc3339(t, "2026-11-04T09:30:00Z") {
			t.Fatalf("unexpected reservation: %+v", reservation)
		}
	})
}

func TestRepositoryReleaseReservation(t *testing.T) {
	fake := &fakeWarehouseServer{
		reservation: sampleReservationResponse(),
	}

	repository := newTestRepository(t, fake)

	reservation, err := repository.ReleaseReservation(t.Context(), "res-1", "customer cancelled")
	if err != nil {
		t.Fatalf("ReleaseReservation returned error: %v", err)
	}

	if reservation.Status != entities.ReservationActive {
		t.Fatalf("unexpected status: %s", reservation.Status)
	}

	fake.errReleaseReservation = status.Error(
		codes.AlreadyExists, "reservation is already in a different state")

	_, err = repository.ReleaseReservation(t.Context(), "res-1", "customer cancelled")
	if !errors.Is(err, domainerrors.ErrReservationConflict) {
		t.Fatalf("expected ErrReservationConflict, got: %v", err)
	}
}

func TestRepositoryConfirmReservation(t *testing.T) {
	fake := &fakeWarehouseServer{
		errConfirmReservation: status.Error(codes.NotFound, "reservation not found"),
	}

	repository := newTestRepository(t, fake)

	_, err := repository.ConfirmReservation(t.Context(), "res-1")
	if !errors.Is(err, domainerrors.ErrReservationNotFound) {
		t.Fatalf("expected ErrReservationNotFound, got: %v", err)
	}
}

func TestRepositoryUnknownStatusesPassThrough(t *testing.T) {
	fake := &fakeWarehouseServer{
		errGetInventory: status.Error(codes.Unavailable, "connection refused"),
	}

	repository := newTestRepository(t, fake)

	_, err := repository.FindByProductID(t.Context(), 7)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected the status error to pass through, got: %v", err)
	}
}
