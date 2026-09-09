package repositories

import (
	"remotehelpdesk/internal/models"
	"time"

	"gorm.io/gorm"
)

var MeetingRoomRepository = &meetingRoomRepository{}

type meetingRoomRepository struct{}

type MeetingAttendanceStats struct {
	Count         int64      `gorm:"column:attendee_count"`
	FirstJoinedAt *time.Time `gorm:"column:first_joined_at"`
}

func (r *meetingRoomRepository) Create(db *gorm.DB, item *models.MeetingRoom) error {
	return db.Create(item).Error
}

func (r *meetingRoomRepository) FindJitsiByTicketIDs(db *gorm.DB, tenantID int64, ticketIDs []string) []models.MeetingRoomJitsi {
	if tenantID <= 0 || len(ticketIDs) == 0 {
		return nil
	}
	var list []models.MeetingRoomJitsi
	db.Where("tenant_id = ? AND ticket_id IN ?", tenantID, ticketIDs).
		Order("created_at DESC").
		Find(&list)
	return list
}

func (r *meetingRoomRepository) FindLatestJitsiByTicketID(db *gorm.DB, tenantID int64, ticketID string) *models.MeetingRoomJitsi {
	if tenantID <= 0 || ticketID == "" {
		return nil
	}
	var meeting models.MeetingRoomJitsi
	if err := db.Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Order("created_at DESC").First(&meeting).Error; err != nil {
		return nil
	}
	return &meeting
}

func (r *meetingRoomRepository) CountParticipants(db *gorm.DB, meetingID string) int64 {
	if meetingID == "" {
		return 0
	}
	var count int64
	db.Model(&models.MeetingParticipant{}).Where("meeting_id = ?", meetingID).Count(&count)
	return count
}

// CountAttendedParticipants excludes identities that only requested a join
// token but were never confirmed by the meeting provider.
func (r *meetingRoomRepository) CountAttendedParticipants(db *gorm.DB, meetingID string) int64 {
	return r.AttendanceStats(db, meetingID).Count
}

func (r *meetingRoomRepository) AttendanceStats(db *gorm.DB, meetingID string) MeetingAttendanceStats {
	stats := MeetingAttendanceStats{}
	if meetingID == "" {
		return stats
	}
	db.Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND joined_at IS NOT NULL", meetingID).
		Count(&stats.Count)
	var first models.MeetingParticipant
	result := db.Select("joined_at").
		Where("meeting_id = ? AND joined_at IS NOT NULL", meetingID).
		Order("joined_at ASC").
		Limit(1).
		Find(&first)
	if result.Error == nil && result.RowsAffected > 0 {
		stats.FirstJoinedAt = first.JoinedAt
	}
	return stats
}

func (r *meetingRoomRepository) AttendanceStatsByMeetingIDs(db *gorm.DB, meetingIDs []string) map[string]MeetingAttendanceStats {
	stats := make(map[string]MeetingAttendanceStats, len(meetingIDs))
	if len(meetingIDs) == 0 {
		return stats
	}
	var participants []models.MeetingParticipant
	db.Select("meeting_id", "joined_at").
		Where("meeting_id IN ? AND joined_at IS NOT NULL", meetingIDs).
		Order("meeting_id ASC, joined_at ASC").
		Find(&participants)
	for i := range participants {
		item := stats[participants[i].MeetingID]
		item.Count++
		if item.FirstJoinedAt == nil && participants[i].JoinedAt != nil {
			joinedAt := *participants[i].JoinedAt
			item.FirstJoinedAt = &joinedAt
		}
		stats[participants[i].MeetingID] = item
	}
	return stats
}

func (r *meetingRoomRepository) ListAttendedParticipantsByMeetingIDs(db *gorm.DB, meetingIDs []string) []models.MeetingParticipant {
	if len(meetingIDs) == 0 {
		return []models.MeetingParticipant{}
	}
	var participants []models.MeetingParticipant
	db.Where("meeting_id IN ? AND joined_at IS NOT NULL", meetingIDs).
		Order("meeting_id ASC, joined_at ASC, created_at ASC").
		Find(&participants)
	return participants
}

func (r *meetingRoomRepository) CountOnlineParticipants(db *gorm.DB, meetingIDs []string) int64 {
	if len(meetingIDs) == 0 {
		return 0
	}
	var count int64
	db.Model(&models.MeetingParticipant{}).
		Where("meeting_id IN ? AND joined_at IS NOT NULL AND left_at IS NULL", meetingIDs).
		Count(&count)
	return count
}

func (r *meetingRoomRepository) EndActiveJitsiByTicket(db *gorm.DB, tenantID int64, ticketID string, endedAt time.Time) ([]models.MeetingRoomJitsi, error) {
	var meetings []models.MeetingRoomJitsi
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND status IN ?", tenantID, ticketID, []string{"waiting", "scheduled", "active"}).Find(&meetings).Error; err != nil {
		return nil, err
	}
	if len(meetings) == 0 {
		return meetings, nil
	}
	meetingIDs := make([]string, 0, len(meetings))
	for i := range meetings {
		meetingIDs = append(meetingIDs, meetings[i].ID)
	}
	if err := db.Model(&models.MeetingRoomJitsi{}).Where("id IN ?", meetingIDs).Updates(map[string]any{
		"status": "ended", "ended_at": endedAt, "updated_at": endedAt,
	}).Error; err != nil {
		return nil, err
	}
	var participants []models.MeetingParticipant
	if err := db.Where("meeting_id IN ? AND left_at IS NULL", meetingIDs).Find(&participants).Error; err != nil {
		return nil, err
	}
	for i := range participants {
		duration := int64(0)
		if participants[i].JoinedAt != nil && endedAt.After(*participants[i].JoinedAt) {
			duration = int64(endedAt.Sub(*participants[i].JoinedAt).Seconds())
		}
		if err := db.Model(&models.MeetingParticipant{}).Where("id = ?", participants[i].ID).Updates(map[string]any{
			"left_at": endedAt, "duration": duration, "updated_at": endedAt,
		}).Error; err != nil {
			return nil, err
		}
	}
	return meetings, nil
}
