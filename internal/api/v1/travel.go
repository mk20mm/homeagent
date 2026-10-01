package v1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	domtravel "github.com/mk20mm/homeagent/internal/domain/travel"
)

type TravelService interface {
	CreateVehicle(ctx context.Context, cmd domtravel.CreateVehicleCmd) (domtravel.Vehicle, error)
	ListVehicles(ctx context.Context) ([]domtravel.Vehicle, error)
	RecordTrip(ctx context.Context, memberID string, cmd domtravel.RecordTripCmd) (domtravel.TripRecord, error)
	ListTrips(ctx context.Context, vehicleID string, limit int) ([]domtravel.TripRecord, error)
}

func ListVehicles(svc TravelService) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := svc.ListVehicles(c.Request.Context())
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		if list == nil {
			list = []domtravel.Vehicle{}
		}
		c.JSON(http.StatusOK, gin.H{"items": list})
	}
}

type createVehicleReq struct {
	Name           string `json:"name" binding:"required"`
	PlateNumber    string `json:"plate_number" binding:"required"`
	VehicleType    string `json:"vehicle_type" binding:"required"`
	CurrentMileage int    `json:"current_mileage"`
	Note           string `json:"note"`
}

func CreateVehicle(svc TravelService, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "expense.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无车辆管理权限", nil))
			return
		}

		var req createVehicleReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数错误", err))
			return
		}

		v, err := svc.CreateVehicle(c.Request.Context(), domtravel.CreateVehicleCmd{
			Name:           req.Name,
			PlateNumber:    req.PlateNumber,
			VehicleType:    req.VehicleType,
			CurrentMileage: req.CurrentMileage,
			Note:           req.Note,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusCreated, v)
	}
}

func ListTrips(svc TravelService) gin.HandlerFunc {
	return func(c *gin.Context) {
		vehID := c.Query("vehicle_id")
		limit := 20
		if lStr := c.Query("limit"); lStr != "" {
			if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
				limit = l
			}
		}

		trips, err := svc.ListTrips(c.Request.Context(), vehID, limit)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		if trips == nil {
			trips = []domtravel.TripRecord{}
		}
		c.JSON(http.StatusOK, gin.H{"items": trips})
	}
}

type createTripReq struct {
	VehicleID   string `json:"vehicle_id"`
	TripType    string `json:"trip_type" binding:"required"`
	AmountCents int64  `json:"amount_cents"`
	Mileage     int    `json:"mileage"`
	Note        string `json:"note"`
	OccurredAt  string `json:"occurred_at"`
}

func CreateTrip(svc TravelService, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "expense.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无出行记录权限", nil))
			return
		}

		var req createTripReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数错误", err))
			return
		}

		occur := time.Now()
		if req.OccurredAt != "" {
			if pt, perr := time.Parse(time.RFC3339, req.OccurredAt); perr == nil {
				occur = pt
			}
		}

		trip, err := svc.RecordTrip(c.Request.Context(), memberID, domtravel.RecordTripCmd{
			VehicleID:   req.VehicleID,
			TripType:    req.TripType,
			AmountCents: req.AmountCents,
			Mileage:     req.Mileage,
			Note:        req.Note,
			OccurredAt:  occur,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusCreated, trip)
	}
}
