package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	VvipStatusNone     = "none"
	VvipStatusActive   = "active"
	VvipStatusDisabled = "disabled"
)

// UserProfile 对应 user_profiles 表，承载用户手机号和 VVIP 状态等业务扩展资料。
type UserProfile struct {
	Id              int     `json:"id" gorm:"comment:主键ID"`
	UserId          int     `json:"user_id" gorm:"uniqueIndex;comment:用户ID"`
	PhoneNumber     *string `json:"phone_number,omitempty" gorm:"type:varchar(32);uniqueIndex;comment:手机号"`
	PhoneVerifiedAt int64   `json:"phone_verified_at" gorm:"bigint;default:0;comment:手机号验证时间戳"`
	IsVvip          bool    `json:"is_vvip" gorm:"default:false;index;comment:是否VVIP快照"`
	VvipActivatedAt int64   `json:"vvip_activated_at" gorm:"bigint;default:0;comment:VVIP开通时间戳"`
	VvipDisabledAt  int64   `json:"vvip_disabled_at" gorm:"bigint;default:0;comment:VVIP禁用时间戳"`
	VvipStatus      string  `json:"vvip_status" gorm:"type:varchar(32);default:'disabled';index;comment:VVIP状态"`
	CreatedAt       int64   `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt       int64   `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// BeforeCreate 初始化用户扩展资料的时间戳和默认 VVIP 状态。
func (p *UserProfile) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if p.VvipStatus == "" {
		p.VvipStatus = VvipStatusDisabled
	}
	return nil
}

// BeforeUpdate 在用户扩展资料更新时刷新更新时间戳。
func (p *UserProfile) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// GetUserProfileByUserId 根据用户 ID 查询用户扩展资料。
func GetUserProfileByUserId(userId int) (*UserProfile, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var profile UserProfile
	if err := DB.Where("user_id = ?", userId).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

// IsPhoneNumberAlreadyTaken 判断手机号是否已被用户绑定。
func IsPhoneNumberAlreadyTaken(phoneNumber string) bool {
	if phoneNumber == "" {
		return false
	}
	return DB.Where("phone_number = ?", phoneNumber).Find(&UserProfile{}).RowsAffected > 0
}

// GetUserProfileByPhoneNumber 根据手机号查询用户扩展资料。
func GetUserProfileByPhoneNumber(phoneNumber string) (*UserProfile, error) {
	return GetUserProfileByPhoneNumberWithTx(DB, phoneNumber)
}

// GetUserProfileByPhoneNumberWithTx 根据手机号在指定事务内查询用户扩展资料。
func GetUserProfileByPhoneNumberWithTx(tx *gorm.DB, phoneNumber string) (*UserProfile, error) {
	if phoneNumber == "" {
		return nil, errors.New("phone number is empty")
	}
	if tx == nil {
		tx = DB
	}
	var profile UserProfile
	if err := tx.Where("phone_number = ?", phoneNumber).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}
