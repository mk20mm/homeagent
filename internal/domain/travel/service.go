// Package travel 家庭出行与爱车台账领域服务（AI-PRD §8.8 M6 出行）。
//
// 不变量：
//   - 金额一律 int64 分；元→分转换只在工具/契约边界做
//   - 出行费用（加油/充电/停车/保养/高速）自动联动生成「交通」分类支出
//   - 写操作通过 undo_log 可撤销
package travel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
	domexp "github.com/mk20mm/homeagent/internal/domain/expense"
)

// Vehicle 领域实体：爱车台账。
type Vehicle struct {
	ID                     string
	Name                   string
	PlateNumber            string
	VehicleType            string // ev | gas | hybrid
	CurrentMileage         int
	LastMaintenanceMileage int
	NextMaintenanceMileage int
	Note                   string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// CreateVehicleCmd 登记车辆。
type CreateVehicleCmd struct {
	Name           string
	PlateNumber    string
	VehicleType    string
	CurrentMileage int
	Note           string
}

// TripRecord 领域实体：出行/用车记录流水。
type TripRecord struct {
	ID          string
	VehicleID   string
	VehicleName string
	MemberID    string
	TripType    string // gas | charging | parking | toll | maintenance | ride
	AmountCents int64
	Mileage     int
	Note        string
	ExpenseID   string
	OccurredAt  time.Time
	CreatedAt   time.Time
}

// RecordTripCmd 记出行/用车命令。
type RecordTripCmd struct {
	VehicleID   string
	TripType    string
	AmountCents int64
	Mileage     int
	Note        string
	OccurredAt  time.Time
}

func (cmd RecordTripCmd) OccurredOrNow() time.Time {
	if cmd.OccurredAt.IsZero() {
		return time.Now()
	}
	return cmd.OccurredAt
}

// TripExpenseRecorder 联动记账接口（避免直接依赖 concrete ExpenseService）。
type TripExpenseRecorder interface {
	RecordExpense(ctx context.Context, cmd domexp.RecordExpenseCmd, memberID string) (string, string, bool, error)
	DeleteExpense(ctx context.Context, expenseID string) error
}

// Repo 仓储接口：领域层定义，store 层实现。
type Repo interface {
	CreateVehicle(ctx context.Context, cmd CreateVehicleCmd) (Vehicle, error)
	GetVehicle(ctx context.Context, id string) (Vehicle, error)
	ListVehicles(ctx context.Context) ([]Vehicle, error)
	UpdateVehicleMileage(ctx context.Context, vehicleID string, mileage int) error
	CreateTrip(ctx context.Context, memberID string, cmd RecordTripCmd, expenseID string, idempotencyKey string) (TripRecord, error)
	GetTrip(ctx context.Context, tripID string) (TripRecord, error)
	ListTrips(ctx context.Context, vehicleID string, limit int) ([]TripRecord, error)
	DeleteTrip(ctx context.Context, tripID string) error
}

// Service 出行领域服务接口。
type Service interface {
	CreateVehicle(ctx context.Context, cmd CreateVehicleCmd) (Vehicle, error)
	ListVehicles(ctx context.Context) ([]Vehicle, error)
	RecordTrip(ctx context.Context, memberID string, cmd RecordTripCmd) (TripRecord, error)
	ListTrips(ctx context.Context, vehicleID string, limit int) ([]TripRecord, error)
	UndoTrip(ctx context.Context, tripID string) error
}

type service struct {
	repo       Repo
	expenseSvc TripExpenseRecorder
}

// NewService 创建出行领域服务。
func NewService(repo Repo, expRec TripExpenseRecorder) Service {
	return &service{
		repo:       repo,
		expenseSvc: expRec,
	}
}

func (s *service) CreateVehicle(ctx context.Context, cmd CreateVehicleCmd) (Vehicle, error) {
	if cmd.Name == "" {
		return Vehicle{}, apperr.New(apperr.CodeInvalidInput, "车辆名称不能为空", nil)
	}
	return s.repo.CreateVehicle(ctx, cmd)
}

func (s *service) ListVehicles(ctx context.Context) ([]Vehicle, error) {
	return s.repo.ListVehicles(ctx)
}

func (s *service) RecordTrip(ctx context.Context, memberID string, cmd RecordTripCmd) (TripRecord, error) {
	if cmd.TripType == "" {
		cmd.TripType = "ride"
	}
	occur := cmd.OccurredOrNow()

	// 幂等键
	raw := fmt.Sprintf("%s:%s:%d:%d", cmd.VehicleID, cmd.TripType, cmd.AmountCents, occur.Unix())
	sum := sha256.Sum256([]byte(raw))
	idempotencyKey := hex.EncodeToString(sum[:])

	// 若有费用且注入了财务记账服务，自动联动记账（归类为「交通」）
	var linkedExpenseID string
	if cmd.AmountCents > 0 && s.expenseSvc != nil {
		hint := formatTripHint(cmd.TripType, cmd.Note)
		expID, _, _, err := s.expenseSvc.RecordExpense(ctx, domexp.RecordExpenseCmd{
			AmountCents: cmd.AmountCents,
			Hint:        hint,
			Category:    "交通",
			OccurredAt:  occur,
		}, memberID)
		if err == nil {
			linkedExpenseID = expID
		}
	}

	// 写入出行记录
	trip, err := s.repo.CreateTrip(ctx, memberID, cmd, linkedExpenseID, idempotencyKey)
	if err != nil {
		return TripRecord{}, err
	}

	// 如果记录中包含大于当前车辆里程的有效数值，同步刷新车辆总里程
	if cmd.VehicleID != "" && cmd.Mileage > 0 {
		_ = s.repo.UpdateVehicleMileage(ctx, cmd.VehicleID, cmd.Mileage)
	}

	return trip, nil
}

func (s *service) ListTrips(ctx context.Context, vehicleID string, limit int) ([]TripRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListTrips(ctx, vehicleID, limit)
}

func (s *service) UndoTrip(ctx context.Context, tripID string) error {
	trip, err := s.repo.GetTrip(ctx, tripID)
	if err != nil {
		return err
	}
	// 联动软删除财务支出
	if trip.ExpenseID != "" && s.expenseSvc != nil {
		_ = s.expenseSvc.DeleteExpense(ctx, trip.ExpenseID)
	}
	return s.repo.DeleteTrip(ctx, tripID)
}

func formatTripHint(tripType, note string) string {
	typeName := tripType
	switch tripType {
	case "gas":
		typeName = "加油"
	case "charging":
		typeName = "充电"
	case "parking":
		typeName = "停车费"
	case "toll":
		typeName = "高速通行费"
	case "maintenance":
		typeName = "车辆保养"
	case "ride":
		typeName = "出行交通"
	}
	if note != "" {
		return fmt.Sprintf("%s (%s)", typeName, note)
	}
	return typeName
}
