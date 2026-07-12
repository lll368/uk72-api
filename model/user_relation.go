package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	UserRelationSourceRegister = "register"
	UserRelationSourceAdmin    = "admin"
)

const (
	UserRelationStatusActive   = "active"
	UserRelationStatusDisabled = "disabled"
)

const (
	UserTopupDiscountSourceNone = "none"
	UserTopupDiscountSourceUser = "user"
)

var (
	ErrUserRelationSelfBinding  = errors.New("user relation self binding")
	ErrUserRelationAlreadyBound = errors.New("user relation already bound")
	ErrUserRelationCycle        = errors.New("user relation cycle")
	ErrUserRelationNotFound     = errors.New("user relation not found")
)

// UserRelation records active and historical VVIP parent-child bindings.
type UserRelation struct {
	Id            int     `json:"id" gorm:"comment:primary key"`
	ParentUserId  int     `json:"parent_user_id" gorm:"index;comment:parent user id"`
	ChildUserId   int     `json:"child_user_id" gorm:"index;comment:child user id"`
	ActiveChildId *int    `json:"-" gorm:"uniqueIndex:idx_user_relations_active_child;comment:active child user id"`
	Source        string  `json:"source" gorm:"type:varchar(32);default:'register';index;comment:relation source"`
	SourceTradeNo string  `json:"source_trade_no" gorm:"type:varchar(255);default:'';index;comment:source trade no"`
	Status        string  `json:"status" gorm:"type:varchar(32);default:'active';index;comment:relation status"`
	TopupDiscount float64 `json:"topup_discount" gorm:"type:decimal(10,6);not null;default:0;comment:legacy subordinate topup discount"`
	BindTime      int64   `json:"bind_time" gorm:"bigint;index;comment:bind timestamp"`
	CreatedAt     int64   `json:"created_at" gorm:"bigint;index;comment:create timestamp"`
	UpdatedAt     int64   `json:"updated_at" gorm:"bigint;comment:update timestamp"`
}

// UserSubordinate describes one direct subordinate visible to a VVIP user.
type UserSubordinate struct {
	RelationId    int     `json:"relation_id" gorm:"column:relation_id"`
	ChildUserId   int     `json:"child_user_id" gorm:"column:child_user_id"`
	Username      string  `json:"username" gorm:"column:username"`
	DisplayName   string  `json:"display_name" gorm:"column:display_name"`
	Status        int     `json:"status" gorm:"column:status"`
	Group         string  `json:"group" gorm:"column:user_group"`
	Quota         int     `json:"quota" gorm:"column:quota"`
	UsedQuota     int     `json:"used_quota" gorm:"column:used_quota"`
	BindTime      int64   `json:"bind_time" gorm:"column:bind_time"`
	TopupDiscount float64 `json:"topup_discount" gorm:"column:topup_discount"`
}

func (r *UserRelation) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if r.BindTime == 0 {
		r.BindTime = now
	}
	if r.Source == "" {
		r.Source = UserRelationSourceRegister
	}
	if r.Status == "" {
		r.Status = UserRelationStatusActive
	}
	r.syncActiveChildId()
	return nil
}

func (r *UserRelation) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	r.syncActiveChildId()
	return nil
}

func (r *UserRelation) syncActiveChildId() {
	if r.Status == UserRelationStatusActive {
		childUserId := r.ChildUserId
		r.ActiveChildId = &childUserId
		return
	}
	r.ActiveChildId = nil
}

func GetUserRelationByChildId(childUserId int) (*UserRelation, error) {
	return GetActiveUserRelationByChildId(childUserId)
}

func GetActiveUserRelationByChildId(childUserId int) (*UserRelation, error) {
	if childUserId <= 0 {
		return nil, errors.New("invalid child user id")
	}
	var relation UserRelation
	if err := DB.Where("child_user_id = ? AND status = ?", childUserId, UserRelationStatusActive).First(&relation).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

func ListUserRelations(pageInfo *common.PageInfo, parentUserId int, childUserId int, status string) (relations []*UserRelation, total int64, err error) {
	query := DB.Model(&UserRelation{})
	if parentUserId > 0 {
		query = query.Where("parent_user_id = ?", parentUserId)
	}
	if childUserId > 0 {
		query = query.Where("child_user_id = ?", childUserId)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&relations).Error
	return relations, total, err
}

func ListActiveUserSubordinates(pageInfo *common.PageInfo, parentUserId int) (subordinates []*UserSubordinate, total int64, err error) {
	if parentUserId <= 0 {
		return nil, 0, errors.New("invalid parent user id")
	}
	query := DB.Model(&UserRelation{}).
		Where("parent_user_id = ? AND status = ?", parentUserId, UserRelationStatusActive)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	selectClause := fmt.Sprintf(
		"user_relations.id AS relation_id, user_relations.child_user_id, users.username, users.display_name, users.status, users.%s AS user_group, users.quota, users.used_quota, user_relations.bind_time, COALESCE(users.topup_discount, 1) AS topup_discount",
		commonGroupCol,
	)
	err = DB.Table("user_relations").
		Select(selectClause).
		Joins("JOIN users ON users.id = user_relations.child_user_id").
		Where("user_relations.parent_user_id = ? AND user_relations.status = ?", parentUserId, UserRelationStatusActive).
		Order("user_relations.id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&subordinates).Error
	return subordinates, total, err
}

func DisableUserRelationById(id int) (*UserRelation, error) {
	if id <= 0 {
		return nil, errors.New("invalid user relation id")
	}
	var relation UserRelation
	if err := DB.Where("id = ?", id).First(&relation).Error; err != nil {
		return nil, err
	}
	if relation.Status == UserRelationStatusDisabled {
		return &relation, nil
	}
	relation.Status = UserRelationStatusDisabled
	relation.ActiveChildId = nil
	if err := DB.Save(&relation).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

func GetEffectiveUserTopupDiscountTx(tx *gorm.DB, userId int) (float64, error) {
	discount, _, err := GetEffectiveUserTopupDiscountWithSourceTx(tx, userId)
	return discount, err
}

func GetEffectiveUserTopupDiscountWithSourceTx(tx *gorm.DB, userId int) (float64, string, error) {
	discount, ok, err := GetUserTopupDiscountTx(tx, userId)
	if err != nil {
		return 1, UserTopupDiscountSourceNone, err
	}
	if ok && discount < 1 {
		return discount, UserTopupDiscountSourceUser, nil
	}
	return 1, UserTopupDiscountSourceNone, nil
}

func GetEffectiveUserTopupDiscount(userId int) (float64, error) {
	return GetEffectiveUserTopupDiscountTx(DB, userId)
}

func GetEffectiveUserTopupDiscountWithSource(userId int) (float64, string, error) {
	return GetEffectiveUserTopupDiscountWithSourceTx(DB, userId)
}

// Deprecated: relation-level discounts are migrated to users.topup_discount.
func GetActiveRelationTopupDiscountForChildTx(tx *gorm.DB, childUserId int) (float64, bool, error) {
	return 0, false, nil
}

// Deprecated: relation-level discounts are migrated to users.topup_discount.
func GetActiveRelationTopupDiscountForChild(childUserId int) (float64, bool, error) {
	return 0, false, nil
}

func ValidateSubordinateTopupDiscount(parentEffectiveDiscount float64, childDiscount float64) error {
	if parentEffectiveDiscount <= 0 || parentEffectiveDiscount > 1 {
		parentEffectiveDiscount = 1
	}
	if childDiscount < parentEffectiveDiscount {
		return fmt.Errorf("subordinate discount must be at least current user discount %.6g", parentEffectiveDiscount)
	}
	if childDiscount > 1 {
		return errors.New("subordinate discount must be less than or equal to 1")
	}
	return nil
}

func UpdateDirectSubordinateTopupDiscount(parentUserId int, childUserId int, discount float64) (*UserRelation, error) {
	if parentUserId <= 0 || childUserId <= 0 {
		return nil, ErrUserRelationNotFound
	}
	var relation UserRelation
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("parent_user_id = ? AND child_user_id = ? AND status = ?", parentUserId, childUserId, UserRelationStatusActive).
			First(&relation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserRelationNotFound
			}
			return err
		}
		return UpdateUserTopupDiscountTx(tx, childUserId, &discount)
	})
	if err != nil {
		return nil, err
	}
	relation.TopupDiscount = discount
	return &relation, nil
}

func ResetDirectSubordinateTopupDiscount(parentUserId int, childUserId int) (*UserRelation, error) {
	if parentUserId <= 0 || childUserId <= 0 {
		return nil, ErrUserRelationNotFound
	}
	var relation UserRelation
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("parent_user_id = ? AND child_user_id = ? AND status = ?", parentUserId, childUserId, UserRelationStatusActive).
			First(&relation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserRelationNotFound
			}
			return err
		}
		return UpdateUserTopupDiscountTx(tx, childUserId, nil)
	})
	if err != nil {
		return nil, err
	}
	relation.TopupDiscount = 1
	return &relation, nil
}

func HasActiveUserRelationByChildIdTx(tx *gorm.DB, childUserId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if childUserId <= 0 {
		return false, nil
	}
	var count int64
	if err := tx.Model(&UserRelation{}).
		Where("child_user_id = ? AND status = ?", childUserId, UserRelationStatusActive).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func WouldCreateUserRelationCycleTx(tx *gorm.DB, parentUserId int, childUserId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if parentUserId <= 0 || childUserId <= 0 {
		return false, nil
	}
	visited := map[int]bool{}
	currentUserId := parentUserId
	for currentUserId > 0 {
		if currentUserId == childUserId {
			return true, nil
		}
		if visited[currentUserId] {
			return true, nil
		}
		visited[currentUserId] = true

		var relation UserRelation
		err := tx.Where("child_user_id = ? AND status = ?", currentUserId, UserRelationStatusActive).
			First(&relation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		currentUserId = relation.ParentUserId
	}
	return false, nil
}

func CreateActiveUserRelationTx(tx *gorm.DB, parentUserId int, childUserId int, source string, sourceTradeNo string) (*UserRelation, error) {
	if tx == nil {
		tx = DB
	}
	if parentUserId <= 0 || childUserId <= 0 {
		return nil, errors.New("invalid user relation")
	}
	if parentUserId == childUserId {
		return nil, ErrUserRelationSelfBinding
	}
	exists, err := HasActiveUserRelationByChildIdTx(tx, childUserId)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserRelationAlreadyBound
	}
	hasCycle, err := WouldCreateUserRelationCycleTx(tx, parentUserId, childUserId)
	if err != nil {
		return nil, err
	}
	if hasCycle {
		return nil, ErrUserRelationCycle
	}
	relation := &UserRelation{
		ParentUserId:  parentUserId,
		ChildUserId:   childUserId,
		Source:        source,
		SourceTradeNo: sourceTradeNo,
		Status:        UserRelationStatusActive,
	}
	if err := tx.Create(relation).Error; err != nil {
		return nil, err
	}
	return relation, nil
}
