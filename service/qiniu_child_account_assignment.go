package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	QiniuAccountIdentitySourceParent        = "parent"
	QiniuAccountIdentitySourceUserBinding   = "user_binding"
	QiniuAccountIdentitySourceTaskReserved  = "task_reserved"
	QiniuAccountIdentitySourceAssignedChild = "assigned_child"
)

type QiniuAccountIdentityResolution struct {
	AccountId      int    `json:"account_id"`
	AssignmentMode string `json:"assignment_mode"`
	Source         string `json:"source"`
	UserId         int    `json:"user_id"`
}

type QiniuAccountIdentityRetryableBlocker struct {
	AccountId int
	TaskId    int
	Reason    string
}

func (err *QiniuAccountIdentityRetryableBlocker) Error() string {
	if err == nil || err.Reason == "" {
		return "七牛子账号暂不可用，请稍后重试"
	}
	return err.Reason
}

func ResolveQiniuAccountIdentityForNextToken(ctx context.Context, userId int, reservedChildAccountId int, operatorId int) (*QiniuAccountIdentityResolution, error) {
	if userId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	if reservedChildAccountId < 0 {
		return nil, errors.New("预留子账号 ID 无效")
	}
	setting := operation_setting.GetQiniuKeySetting()
	var resolution *QiniuAccountIdentityResolution
	var blocker *QiniuAccountIdentityRetryableBlocker
	needsCreateTask := false

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		user, err := loadQiniuAssignmentUserForUpdate(tx, userId)
		if err != nil {
			return err
		}
		if !operation_setting.IsQiniuChildAccountBindingEligible(setting, user.CreatedAt) {
			resolution = &QiniuAccountIdentityResolution{
				AccountId:      0,
				AssignmentMode: operation_setting.QiniuChildAccountAssignmentModeParentOnly,
				Source:         QiniuAccountIdentitySourceParent,
				UserId:         user.Id,
			}
			return nil
		}

		if reservedChildAccountId > 0 || user.QiniuChildAccountId > 0 {
			accountId := user.QiniuChildAccountId
			source := QiniuAccountIdentitySourceUserBinding
			if reservedChildAccountId > 0 {
				accountId = reservedChildAccountId
				source = QiniuAccountIdentitySourceTaskReserved
			}
			available, reason, err := qiniuChildAccountAvailableForAssignmentTx(tx, accountId, user.Id)
			if err != nil {
				return err
			}
			if !available {
				blocker = &QiniuAccountIdentityRetryableBlocker{AccountId: accountId, Reason: reason}
				return nil
			}
			if user.QiniuChildAccountId == 0 {
				if err := persistQiniuUserChildAccountBinding(tx, user.Id, 0, accountId); err != nil {
					return err
				}
			}
			resolution = &QiniuAccountIdentityResolution{
				AccountId:      accountId,
				AssignmentMode: operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild,
				Source:         source,
				UserId:         user.Id,
			}
			return nil
		}

		account, err := findAvailableQiniuChildAccountForAssignmentTx(tx)
		if err != nil {
			return err
		}
		if account == nil {
			needsCreateTask = true
			return nil
		}
		if err := persistQiniuUserChildAccountBinding(tx, user.Id, user.QiniuChildAccountId, account.Id); err != nil {
			return err
		}
		resolution = &QiniuAccountIdentityResolution{
			AccountId:      account.Id,
			AssignmentMode: operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild,
			Source:         QiniuAccountIdentitySourceAssignedChild,
			UserId:         user.Id,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if resolution != nil {
		return resolution, nil
	}
	if blocker != nil {
		return nil, blocker
	}
	if needsCreateTask {
		return nil, ensureQiniuChildAccountCreateTaskBlocker(ctx, operatorId)
	}
	return nil, errors.New("七牛子账号分配失败")
}

func loadQiniuAssignmentUserForUpdate(tx *gorm.DB, userId int) (*model.User, error) {
	query := tx
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var user model.User
	if err := query.First(&user, "id = ?", userId).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func findAvailableQiniuChildAccountForAssignmentTx(tx *gorm.DB) (*model.QiniuChildAccount, error) {
	query := tx.Where("status = ?", model.QiniuChildAccountStatusEnabled).Order("sequence_no asc, id asc")
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var accounts []model.QiniuChildAccount
	if err := query.Find(&accounts).Error; err != nil {
		return nil, err
	}
	for _, account := range accounts {
		available, _, err := qiniuChildAccountAvailableForAssignmentTx(tx, account.Id, 0)
		if err != nil {
			return nil, err
		}
		if available {
			return &account, nil
		}
	}
	return nil, nil
}

func qiniuChildAccountAvailableForAssignmentTx(tx *gorm.DB, accountId int, userId int) (bool, string, error) {
	if accountId <= 0 {
		return false, "七牛子账号 ID 无效", nil
	}
	var account model.QiniuChildAccount
	query := tx
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&account, "id = ?", accountId).Error; err != nil {
		return false, "", err
	}
	if account.Status != model.QiniuChildAccountStatusEnabled {
		return false, "七牛子账号未启用", nil
	}
	boundUsers, err := model.ListUsersByQiniuChildAccountIdWithTx(tx, accountId, 2)
	if err != nil {
		return false, "", err
	}
	for _, boundUser := range boundUsers {
		if userId > 0 && boundUser.Id == userId {
			continue
		}
		return false, "七牛子账号已被用户预留", nil
	}
	nonDeletedCount, err := model.CountNonDeletedQiniuManagedTokensByChildAccountIdWithTx(tx, accountId)
	if err != nil {
		return false, "", err
	}
	if nonDeletedCount > 0 {
		return false, "七牛子账号已有未删除托管 Key", nil
	}
	pendingCleanupCount, err := model.CountRemoteCleanupPendingSoftDeletedQiniuTokensByChildAccountIdWithTx(tx, accountId)
	if err != nil {
		return false, "", err
	}
	if pendingCleanupCount > 0 {
		return false, "七牛子账号存在远端清理未完成的历史 Key", nil
	}
	return true, "", nil
}

func persistQiniuUserChildAccountBinding(tx *gorm.DB, userId int, currentAccountId int, nextAccountId int) error {
	result := tx.Model(&model.User{}).
		Where("id = ? AND qiniu_child_account_id = ?", userId, currentAccountId).
		Update("qiniu_child_account_id", nextAccountId)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("七牛子账号绑定已变化，请重试")
	}
	return nil
}

func ensureQiniuChildAccountCreateTaskBlocker(ctx context.Context, operatorId int) *QiniuAccountIdentityRetryableBlocker {
	task, err := latestRetryableQiniuChildAccountCreateTask()
	if err == nil && task != nil {
		return &QiniuAccountIdentityRetryableBlocker{
			AccountId: task.AccountId,
			TaskId:    task.Id,
			Reason:    "七牛子账号创建任务处理中，请稍后重试",
		}
	}
	account, createdTask, createErr := CreateQiniuChildAccount(ctx, operatorId)
	if createErr != nil {
		return &QiniuAccountIdentityRetryableBlocker{Reason: fmt.Sprintf("七牛子账号暂无可用账户: %s", sanitizeQiniuTaskError(createErr))}
	}
	return &QiniuAccountIdentityRetryableBlocker{
		AccountId: account.Id,
		TaskId:    createdTask.Id,
		Reason:    "七牛子账号暂无可用账户，已创建补偿任务",
	}
}

func latestRetryableQiniuChildAccountCreateTask() (*model.QiniuChildAccountSyncTask, error) {
	statuses := []string{
		model.QiniuChildAccountTaskStatusPending,
		model.QiniuChildAccountTaskStatusRunning,
		model.QiniuChildAccountTaskStatusFailed,
	}
	var task model.QiniuChildAccountSyncTask
	err := model.DB.Where("task_type = ? AND status IN ?", model.QiniuChildAccountTaskTypeCreate, statuses).
		Order("id desc").
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}
