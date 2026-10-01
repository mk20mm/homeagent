package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/apperr"
	domtravel "github.com/mk20mm/homeagent/internal/domain/travel"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/expense"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/triprecord"
	"github.com/mk20mm/homeagent/internal/store/ent/vehicle"
)

var _ domtravel.Repo = (*Store)(nil)

func (s *Store) CreateVehicle(ctx context.Context, cmd domtravel.CreateVehicleCmd) (domtravel.Vehicle, error) {
	create := s.db.Vehicle.Create().
		SetName(cmd.Name).
		SetPlateNumber(cmd.PlateNumber).
		SetVehicleType(cmd.VehicleType).
		SetCurrentMileage(cmd.CurrentMileage)
	if cmd.Note != "" {
		create.SetNote(cmd.Note)
	}

	v, err := create.Save(ctx)
	if err != nil {
		return domtravel.Vehicle{}, apperr.New(apperr.CodeInternal, "创建车辆失败", err)
	}

	return domtravel.Vehicle{
		ID:                     v.ID.String(),
		Name:                   v.Name,
		PlateNumber:            v.PlateNumber,
		VehicleType:            v.VehicleType,
		CurrentMileage:         v.CurrentMileage,
		LastMaintenanceMileage: v.LastMaintenanceMileage,
		NextMaintenanceMileage: v.NextMaintenanceMileage,
		Note:                   v.Note,
		CreatedAt:              v.CreatedAt,
		UpdatedAt:              v.UpdatedAt,
	}, nil
}

func (s *Store) GetVehicle(ctx context.Context, id string) (domtravel.Vehicle, error) {
	v, err := s.db.Vehicle.Query().
		Where(vehicle.IDEQ(toUUID(id)), vehicle.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return domtravel.Vehicle{}, apperr.New(apperr.CodeNotFound, "车辆不存在", err)
	}
	return domtravel.Vehicle{
		ID:                     v.ID.String(),
		Name:                   v.Name,
		PlateNumber:            v.PlateNumber,
		VehicleType:            v.VehicleType,
		CurrentMileage:         v.CurrentMileage,
		LastMaintenanceMileage: v.LastMaintenanceMileage,
		NextMaintenanceMileage: v.NextMaintenanceMileage,
		Note:                   v.Note,
		CreatedAt:              v.CreatedAt,
		UpdatedAt:              v.UpdatedAt,
	}, nil
}

func (s *Store) ListVehicles(ctx context.Context) ([]domtravel.Vehicle, error) {
	list, err := s.db.Vehicle.Query().
		Where(vehicle.DeletedAtIsNil()).
		Order(ent.Asc(vehicle.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询车辆列表失败", err)
	}

	out := make([]domtravel.Vehicle, 0, len(list))
	for _, v := range list {
		out = append(out, domtravel.Vehicle{
			ID:                     v.ID.String(),
			Name:                   v.Name,
			PlateNumber:            v.PlateNumber,
			VehicleType:            v.VehicleType,
			CurrentMileage:         v.CurrentMileage,
			LastMaintenanceMileage: v.LastMaintenanceMileage,
			NextMaintenanceMileage: v.NextMaintenanceMileage,
			Note:                   v.Note,
			CreatedAt:              v.CreatedAt,
			UpdatedAt:              v.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Store) UpdateVehicleMileage(ctx context.Context, vehicleID string, mileage int) error {
	v, err := s.db.Vehicle.Query().
		Where(vehicle.IDEQ(toUUID(vehicleID)), vehicle.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return err
	}
	if mileage <= v.CurrentMileage {
		return nil
	}
	_, err = s.db.Vehicle.UpdateOne(v).
		SetCurrentMileage(mileage).
		Save(ctx)
	return err
}

func (s *Store) CreateTrip(ctx context.Context, memberID string, cmd domtravel.RecordTripCmd, expenseID string, idempotencyKey string) (domtravel.TripRecord, error) {
	m, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(memberID))).
		Only(ctx)
	if err != nil {
		return domtravel.TripRecord{}, apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}

	create := s.db.TripRecord.Create().
		SetTripType(cmd.TripType).
		SetAmountCents(cmd.AmountCents).
		SetMileage(cmd.Mileage).
		SetOccurredAt(cmd.OccurredOrNow()).
		SetMember(m)

	if cmd.Note != "" {
		create.SetNote(cmd.Note)
	}
	if idempotencyKey != "" {
		create.SetIdempotencyKey(idempotencyKey)
	}

	var vehicleName string
	if cmd.VehicleID != "" {
		vehUUID, uerr := uuid.Parse(cmd.VehicleID)
		if uerr == nil {
			v, verr := s.db.Vehicle.Query().Where(vehicle.IDEQ(vehUUID)).Only(ctx)
			if verr == nil && v != nil {
				create.SetVehicle(v)
				vehicleName = v.Name
			}
		}
	}

	if expenseID != "" {
		expUUID, uerr := uuid.Parse(expenseID)
		if uerr == nil {
			exp, eerr := s.db.Expense.Query().Where(expense.IDEQ(expUUID)).Only(ctx)
			if eerr == nil && exp != nil {
				create.SetExpense(exp)
			}
		}
	}

	saved, err := create.Save(ctx)
	if err != nil {
		return domtravel.TripRecord{}, apperr.New(apperr.CodeInternal, "创建出行记录失败", err)
	}

	return domtravel.TripRecord{
		ID:          saved.ID.String(),
		VehicleID:   cmd.VehicleID,
		VehicleName: vehicleName,
		MemberID:    memberID,
		TripType:    saved.TripType,
		AmountCents: saved.AmountCents,
		Mileage:     saved.Mileage,
		Note:        saved.Note,
		ExpenseID:   expenseID,
		OccurredAt:  saved.OccurredAt,
		CreatedAt:   saved.CreatedAt,
	}, nil
}

func (s *Store) GetTrip(ctx context.Context, tripID string) (domtravel.TripRecord, error) {
	t, err := s.db.TripRecord.Query().
		Where(triprecord.IDEQ(toUUID(tripID)), triprecord.DeletedAtIsNil()).
		WithVehicle().
		WithMember().
		WithExpense().
		Only(ctx)
	if err != nil {
		return domtravel.TripRecord{}, apperr.New(apperr.CodeNotFound, "出行记录不存在", err)
	}

	var vehID, vehName, memID, expID string
	if t.Edges.Vehicle != nil {
		vehID = t.Edges.Vehicle.ID.String()
		vehName = t.Edges.Vehicle.Name
	}
	if t.Edges.Member != nil {
		memID = t.Edges.Member.ID.String()
	}
	if t.Edges.Expense != nil {
		expID = t.Edges.Expense.ID.String()
	}

	return domtravel.TripRecord{
		ID:          t.ID.String(),
		VehicleID:   vehID,
		VehicleName: vehName,
		MemberID:    memID,
		TripType:    t.TripType,
		AmountCents: t.AmountCents,
		Mileage:     t.Mileage,
		Note:        t.Note,
		ExpenseID:   expID,
		OccurredAt:  t.OccurredAt,
		CreatedAt:   t.CreatedAt,
	}, nil
}

func (s *Store) ListTrips(ctx context.Context, vehicleID string, limit int) ([]domtravel.TripRecord, error) {
	q := s.db.TripRecord.Query().
		Where(triprecord.DeletedAtIsNil()).
		WithVehicle().
		WithMember().
		WithExpense().
		Order(ent.Desc(triprecord.FieldOccurredAt)).
		Limit(limit)

	if vehicleID != "" {
		q = q.Where(triprecord.HasVehicleWith(vehicle.IDEQ(toUUID(vehicleID))))
	}

	list, err := q.All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询出行记录失败", err)
	}

	out := make([]domtravel.TripRecord, 0, len(list))
	for _, t := range list {
		var vehID, vehName, memID, expID string
		if t.Edges.Vehicle != nil {
			vehID = t.Edges.Vehicle.ID.String()
			vehName = t.Edges.Vehicle.Name
		}
		if t.Edges.Member != nil {
			memID = t.Edges.Member.ID.String()
		}
		if t.Edges.Expense != nil {
			expID = t.Edges.Expense.ID.String()
		}
		out = append(out, domtravel.TripRecord{
			ID:          t.ID.String(),
			VehicleID:   vehID,
			VehicleName: vehName,
			MemberID:    memID,
			TripType:    t.TripType,
			AmountCents: t.AmountCents,
			Mileage:     t.Mileage,
			Note:        t.Note,
			ExpenseID:   expID,
			OccurredAt:  t.OccurredAt,
			CreatedAt:   t.CreatedAt,
		})
	}
	return out, nil
}

func (s *Store) DeleteTrip(ctx context.Context, tripID string) error {
	return s.db.TripRecord.Update().
		Where(triprecord.IDEQ(toUUID(tripID))).
		SetDeletedAt(time.Now()).
		Exec(ctx)
}
