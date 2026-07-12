package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	ContactMessageStatusPending     = "pending"
	ContactMessageStatusContacted   = "contacted"
	ContactMessageStatusUnreachable = "unreachable"
)

var ErrContactMessageNotFound = errors.New("留言记录不存在")

// ContactMessage 对应 contact_messages 表，记录首页访客提交的联系留言。
type ContactMessage struct {
	Id          int    `json:"id" gorm:"comment:主键ID"`
	Name        string `json:"name" gorm:"type:varchar(64);not null;comment:姓名"`
	Phone       string `json:"phone" gorm:"type:varchar(32);not null;index;comment:联系电话"`
	Message     string `json:"message" gorm:"type:text;comment:留言内容"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index;comment:处理状态"`
	Remark      string `json:"remark" gorm:"type:text;comment:管理员备注"`
	ProcessedAt int64  `json:"processed_at" gorm:"bigint;default:0;index;comment:最近处理时间戳"`
	ProcessedBy int    `json:"processed_by" gorm:"type:int;default:0;index;comment:最近处理管理员ID"`
	ClientIp    string `json:"client_ip" gorm:"type:varchar(64);default:'';comment:提交客户端IP"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// IsValidContactMessageStatus 判断留言状态是否属于允许的业务枚举。
func IsValidContactMessageStatus(status string) bool {
	switch status {
	case ContactMessageStatusPending, ContactMessageStatusContacted, ContactMessageStatusUnreachable:
		return true
	default:
		return false
	}
}

// BeforeCreate 初始化留言的默认状态和时间戳。
func (m *ContactMessage) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.Status == "" {
		m.Status = ContactMessageStatusPending
	}
	return nil
}

// BeforeUpdate 在留言更新时刷新更新时间戳。
func (m *ContactMessage) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = common.GetTimestamp()
	return nil
}

// CreateContactMessage 创建一条首页访客留言。
func CreateContactMessage(message *ContactMessage) error {
	if message == nil {
		return errors.New("留言记录不能为空")
	}
	if !IsValidContactMessageStatus(message.Status) && message.Status != "" {
		return errors.New("留言状态无效")
	}
	return DB.Create(message).Error
}

// GetContactMessageById 根据 ID 查询留言。
func GetContactMessageById(id int) (*ContactMessage, error) {
	if id <= 0 {
		return nil, ErrContactMessageNotFound
	}
	var message ContactMessage
	if err := DB.Where("id = ?", id).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContactMessageNotFound
		}
		return nil, err
	}
	return &message, nil
}

// ListContactMessages 分页查询留言记录，可按状态筛选。
func ListContactMessages(pageInfo *common.PageInfo, status string) (messages []*ContactMessage, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&ContactMessage{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&messages).Error
	return messages, total, err
}

// UpdateContactMessageProcessing 更新留言处理状态、备注和处理人信息。
func UpdateContactMessageProcessing(id int, status string, remark string, processedBy int, processedAt int64) (*ContactMessage, error) {
	if id <= 0 {
		return nil, ErrContactMessageNotFound
	}
	if !IsValidContactMessageStatus(status) {
		return nil, errors.New("留言状态无效")
	}
	if processedAt == 0 {
		processedAt = common.GetTimestamp()
	}
	updates := map[string]interface{}{
		"status":       status,
		"remark":       remark,
		"processed_by": processedBy,
		"processed_at": processedAt,
	}
	result := DB.Model(&ContactMessage{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrContactMessageNotFound
	}
	return GetContactMessageById(id)
}

// DeleteContactMessageById 删除指定留言记录。
func DeleteContactMessageById(id int) error {
	if id <= 0 {
		return ErrContactMessageNotFound
	}
	result := DB.Delete(&ContactMessage{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrContactMessageNotFound
	}
	return nil
}
