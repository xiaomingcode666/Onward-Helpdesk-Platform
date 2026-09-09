package services

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var TicketRepairService = newTicketRepairService()

func newTicketRepairService() *ticketRepairService {
	return &ticketRepairService{}
}

type ticketRepairService struct {
}

func (s *ticketRepairService) CreateRepairRecord(record *models.TicketRepairRecord) error {
	return repositories.TicketRepairRepository.Create(sqls.DB(), record)
}

func (s *ticketRepairService) GetRepairRecord(id int64) *models.TicketRepairRecord {
	if id <= 0 {
		return nil
	}
	return repositories.TicketRepairRepository.Get(sqls.DB(), id)
}

func (s *ticketRepairService) FindRepairRecordsByTicket(ticketID int64) []models.TicketRepairRecord {
	return repositories.TicketRepairRepository.FindByTicketID(sqls.DB(), ticketID)
}

func (s *ticketRepairService) FindRepairRecordsByDevice(deviceID int64) []models.TicketRepairRecord {
	return repositories.TicketRepairRepository.FindByDeviceID(sqls.DB(), deviceID)
}

func (s *ticketRepairService) CreateDeviceServiceRecord(record *models.DeviceServiceRecord) error {
	return repositories.DeviceServiceRecordRepository.Create(sqls.DB(), record)
}

func (s *ticketRepairService) GetDeviceServiceRecord(id int64) *models.DeviceServiceRecord {
	if id <= 0 {
		return nil
	}
	return repositories.DeviceServiceRecordRepository.Get(sqls.DB(), id)
}

func (s *ticketRepairService) FindDeviceServiceRecordsByDevice(deviceID int64) []models.DeviceServiceRecord {
	return repositories.DeviceServiceRecordRepository.FindByDeviceID(sqls.DB(), deviceID)
}

func (s *ticketRepairService) FindDeviceServiceRecordsByProduct(productID int64) []models.DeviceServiceRecord {
	return repositories.DeviceServiceRecordRepository.FindByProductID(sqls.DB(), productID)
}

func (s *ticketRepairService) FindDeviceServiceRecordsByTicket(ticketID int64) []models.DeviceServiceRecord {
	return repositories.DeviceServiceRecordRepository.FindByTicketID(sqls.DB(), ticketID)
}

func (s *ticketRepairService) FindDeviceServiceRecordsByCnd(cnd *sqls.Cnd) ([]models.DeviceServiceRecord, *sqls.Paging) {
	return repositories.DeviceServiceRecordRepository.FindPageByCnd(sqls.DB(), cnd)
}
