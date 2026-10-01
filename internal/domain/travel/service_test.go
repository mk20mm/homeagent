package travel

import (
	"context"
	"testing"
	"time"

	domexp "github.com/mk20mm/homeagent/internal/domain/expense"
)

type mockRepo struct {
	vehicles []Vehicle
	trips    []TripRecord
}

func (m *mockRepo) CreateVehicle(ctx context.Context, cmd CreateVehicleCmd) (Vehicle, error) {
	v := Vehicle{
		ID:             "veh-1",
		Name:           cmd.Name,
		PlateNumber:    cmd.PlateNumber,
		VehicleType:    cmd.VehicleType,
		CurrentMileage: cmd.CurrentMileage,
		CreatedAt:      time.Now(),
	}
	m.vehicles = append(m.vehicles, v)
	return v, nil
}

func (m *mockRepo) GetVehicle(ctx context.Context, id string) (Vehicle, error) {
	for _, v := range m.vehicles {
		if v.ID == id {
			return v, nil
		}
	}
	return Vehicle{}, nil
}

func (m *mockRepo) ListVehicles(ctx context.Context) ([]Vehicle, error) {
	return m.vehicles, nil
}

func (m *mockRepo) UpdateVehicleMileage(ctx context.Context, vehicleID string, mileage int) error {
	for i := range m.vehicles {
		if m.vehicles[i].ID == vehicleID {
			m.vehicles[i].CurrentMileage = mileage
		}
	}
	return nil
}

func (m *mockRepo) CreateTrip(ctx context.Context, memberID string, cmd RecordTripCmd, expenseID string, idempotencyKey string) (TripRecord, error) {
	t := TripRecord{
		ID:          "trip-1",
		VehicleID:   cmd.VehicleID,
		MemberID:    memberID,
		TripType:    cmd.TripType,
		AmountCents: cmd.AmountCents,
		Mileage:     cmd.Mileage,
		Note:        cmd.Note,
		ExpenseID:   expenseID,
		OccurredAt:  cmd.OccurredOrNow(),
		CreatedAt:   time.Now(),
	}
	m.trips = append(m.trips, t)
	return t, nil
}

func (m *mockRepo) GetTrip(ctx context.Context, tripID string) (TripRecord, error) {
	for _, t := range m.trips {
		if t.ID == tripID {
			return t, nil
		}
	}
	return TripRecord{}, nil
}

func (m *mockRepo) ListTrips(ctx context.Context, vehicleID string, limit int) ([]TripRecord, error) {
	return m.trips, nil
}

func (m *mockRepo) DeleteTrip(ctx context.Context, tripID string) error {
	var next []TripRecord
	for _, t := range m.trips {
		if t.ID != tripID {
			next = append(next, t)
		}
	}
	m.trips = next
	return nil
}

type mockExpenseRecorder struct {
	recorded []domexp.RecordExpenseCmd
	deleted  []string
}

func (m *mockExpenseRecorder) RecordExpense(ctx context.Context, cmd domexp.RecordExpenseCmd, memberID string) (string, string, bool, error) {
	m.recorded = append(m.recorded, cmd)
	return "exp-linked-1", cmd.Category, false, nil
}

func (m *mockExpenseRecorder) DeleteExpense(ctx context.Context, expenseID string) error {
	m.deleted = append(m.deleted, expenseID)
	return nil
}

func TestTravelService_VehicleAndTrip(t *testing.T) {
	repo := &mockRepo{}
	expRec := &mockExpenseRecorder{}
	svc := NewService(repo, expRec)

	ctx := context.Background()

	// 1. 登记车辆
	veh, err := svc.CreateVehicle(ctx, CreateVehicleCmd{
		Name:           "Model Y",
		PlateNumber:    "京A·88888",
		VehicleType:    "ev",
		CurrentMileage: 20000,
	})
	if err != nil {
		t.Fatalf("CreateVehicle failed: %v", err)
	}
	if veh.Name != "Model Y" {
		t.Errorf("expected Model Y, got %s", veh.Name)
	}

	// 2. 记录一次充电出行（带费用）
	trip, err := svc.RecordTrip(ctx, "mem-1", RecordTripCmd{
		VehicleID:   veh.ID,
		TripType:    "charging",
		AmountCents: 4500, // 45元
		Mileage:     20350,
		Note:        "特来电快充",
	})
	if err != nil {
		t.Fatalf("RecordTrip failed: %v", err)
	}
	if trip.ExpenseID != "exp-linked-1" {
		t.Errorf("expected linked expense ID, got %s", trip.ExpenseID)
	}
	if len(expRec.recorded) != 1 {
		t.Fatalf("expected 1 recorded expense, got %d", len(expRec.recorded))
	}
	if expRec.recorded[0].Category != "交通" {
		t.Errorf("expected Category 交通, got %s", expRec.recorded[0].Category)
	}
	if expRec.recorded[0].AmountCents != 4500 {
		t.Errorf("expected 4500 cents, got %d", expRec.recorded[0].AmountCents)
	}

	// 3. 撤销出行，应级联撤销财务支出
	err = svc.UndoTrip(ctx, trip.ID)
	if err != nil {
		t.Fatalf("UndoTrip failed: %v", err)
	}
	if len(expRec.deleted) != 1 || expRec.deleted[0] != "exp-linked-1" {
		t.Errorf("expected deleted expense exp-linked-1, got %v", expRec.deleted)
	}
}
