package postgres

import "github.com/jackc/pgx/v5/pgxpool"

type Repository struct {
	*WarehouseRepository
	*ReservationRepository
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		WarehouseRepository:   NewWarehouseRepository(db),
		ReservationRepository: NewReservationRepository(db),
	}
}
