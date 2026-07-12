package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	QiniuKeyTaskTypeDefaultCreate = "qiniu_default_key_create"
	QiniuKeyTaskTypeManualCreate  = "qiniu_key_create"
	QiniuKeyTaskTypeQuotaSync     = "qiniu_key_quota_sync"
	QiniuKeyTaskTypeRevoke        = "qiniu_key_revoke"

	QiniuKeyTaskStatusPending = "pending"
	QiniuKeyTaskStatusRunning = "running"
	QiniuKeyTaskStatusSuccess = "success"
	QiniuKeyTaskStatusFailed  = "failed"
	QiniuKeyTaskStatusSkipped = "skipped"
)

const (
	QiniuRemoteCleanupResultSuccess           = "success"
	QiniuRemoteCleanupResultIdempotentSuccess = "idempotent_success"
)

var qiniuKeyBlockingDefaultTaskStatuses = []string{
	QiniuKeyTaskStatusPending,
	QiniuKeyTaskStatusRunning,
	QiniuKeyTaskStatusFailed,
}

var qiniuKeyCreateTaskTypes = []string{
	QiniuKeyTaskTypeDefaultCreate,
	QiniuKeyTaskTypeManualCreate,
}

// QiniuKeySyncTask 记录七牛 Key 创建、额度同步和远端作废的补偿任务。
type QiniuKeySyncTask struct {
	Id                  int    `json:"id"`
	TaskType            string `json:"task_type" gorm:"type:varchar(64);index:idx_qiniu_task_user_type_status"`
	UserId              int    `json:"user_id" gorm:"index:idx_qiniu_task_user_type_status"`
	TokenId             int    `json:"token_id" gorm:"index:idx_qiniu_task_token_status"`
	QiniuKey            string `json:"qiniu_key" gorm:"type:varchar(128);index"`
	Status              string `json:"status" gorm:"type:varchar(32);index:idx_qiniu_task_user_type_status;index:idx_qiniu_task_token_status"`
	RetryCount          int    `json:"retry_count"`
	NextRetryTime       int64  `json:"next_retry_time" gorm:"bigint;index"`
	LastError           string `json:"last_error" gorm:"type:text"`
	Payload             string `json:"payload" gorm:"type:text"`
	RemoteCleanupResult string `json:"remote_cleanup_result" gorm:"type:varchar(32);default:'';index"`
	CreatedTime         int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64  `json:"updated_time" gorm:"bigint"`
	StartedTime         int64  `json:"started_time" gorm:"bigint"`
	CompletedTime       int64  `json:"completed_time" gorm:"bigint"`
}

func (task *QiniuKeySyncTask) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if task.CreatedTime == 0 {
		task.CreatedTime = now
	}
	if task.UpdatedTime == 0 {
		task.UpdatedTime = now
	}
	if task.Status == "" {
		task.Status = QiniuKeyTaskStatusPending
	}
	if !IsValidQiniuRemoteCleanupResult(task.RemoteCleanupResult) {
		return errors.New("七牛远端清理结果不合法")
	}
	return nil
}

func (task *QiniuKeySyncTask) BeforeUpdate(tx *gorm.DB) error {
	task.UpdatedTime = common.GetTimestamp()
	return nil
}

func CreateQiniuKeySyncTask(task *QiniuKeySyncTask) error {
	if task == nil {
		return errors.New("七牛 Key 同步任务不能为空")
	}
	return DB.Create(task).Error
}

func GetQiniuKeySyncTaskById(id int) (*QiniuKeySyncTask, error) {
	var task QiniuKeySyncTask
	if err := DB.First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func GetLatestQiniuDefaultKeyTask(userId int) (*QiniuKeySyncTask, error) {
	var task QiniuKeySyncTask
	err := DB.Where("user_id = ? AND task_type = ?", userId, QiniuKeyTaskTypeDefaultCreate).
		Order("id desc").
		First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func HasBlockingQiniuDefaultKeyTask(userId int) (bool, error) {
	var count int64
	err := DB.Model(&QiniuKeySyncTask{}).
		Where("user_id = ? AND task_type = ? AND status IN ?", userId, QiniuKeyTaskTypeDefaultCreate, qiniuKeyBlockingDefaultTaskStatuses).
		Count(&count).Error
	return count > 0, err
}

func HasBlockingQiniuCreateTask(userId int) (bool, error) {
	return HasBlockingQiniuCreateTaskWithTx(DB, userId)
}

func HasBlockingQiniuCreateTaskWithTx(tx *gorm.DB, userId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&QiniuKeySyncTask{}).
		Where("user_id = ? AND task_type IN ? AND status IN ?", userId, qiniuKeyCreateTaskTypes, qiniuKeyBlockingDefaultTaskStatuses).
		Count(&count).Error
	return count > 0, err
}

func HasBlockingQiniuRevokeTaskWithTx(tx *gorm.DB, userId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&QiniuKeySyncTask{}).
		Where("user_id = ? AND task_type = ? AND status IN ?", userId, QiniuKeyTaskTypeRevoke, qiniuKeyBlockingDefaultTaskStatuses).
		Count(&count).Error
	return count > 0, err
}

func GetLatestBlockingQiniuRevokeTask(userId int) (*QiniuKeySyncTask, error) {
	var task QiniuKeySyncTask
	err := DB.Where("user_id = ? AND task_type = ? AND status IN ?", userId, QiniuKeyTaskTypeRevoke, qiniuKeyBlockingDefaultTaskStatuses).
		Order("id desc").
		First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func GetLatestQiniuCreateTask(userId int) (*QiniuKeySyncTask, error) {
	var task QiniuKeySyncTask
	err := DB.Where("user_id = ? AND task_type IN ?", userId, qiniuKeyCreateTaskTypes).
		Order("id desc").
		First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func TryMarkQiniuKeyTaskRunning(id int, runningStaleBefore int64) (bool, error) {
	now := common.GetTimestamp()
	result := DB.Model(&QiniuKeySyncTask{}).
		Where(
			"id = ? AND (status IN ? OR (status = ? AND started_time <= ?))",
			id,
			[]string{QiniuKeyTaskStatusPending, QiniuKeyTaskStatusFailed},
			QiniuKeyTaskStatusRunning,
			runningStaleBefore,
		).
		Updates(map[string]interface{}{
			"status":       QiniuKeyTaskStatusRunning,
			"started_time": now,
			"updated_time": now,
		})
	return result.RowsAffected > 0, result.Error
}

func MarkQiniuKeyTaskSuccess(id int, tokenId int, qiniuKey string) error {
	now := common.GetTimestamp()
	return DB.Model(&QiniuKeySyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"token_id":        tokenId,
		"qiniu_key":       qiniuKey,
		"status":          QiniuKeyTaskStatusSuccess,
		"last_error":      "",
		"completed_time":  now,
		"next_retry_time": int64(0),
		"updated_time":    now,
	}).Error
}

func MarkQiniuKeyTaskRemoteKey(id int, qiniuKey string) error {
	now := common.GetTimestamp()
	return DB.Model(&QiniuKeySyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"qiniu_key":    qiniuKey,
		"updated_time": now,
	}).Error
}

func MarkQiniuKeyTaskRemoteCleanupResult(id int, result string) error {
	if !IsValidQiniuRemoteCleanupResult(result) || result == "" {
		return errors.New("七牛远端清理结果不合法")
	}
	now := common.GetTimestamp()
	return DB.Model(&QiniuKeySyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"remote_cleanup_result": result,
		"updated_time":          now,
	}).Error
}

func IsValidQiniuRemoteCleanupResult(result string) bool {
	return result == "" || IsReusableQiniuRemoteCleanupResult(result)
}

func IsReusableQiniuRemoteCleanupResult(result string) bool {
	return result == QiniuRemoteCleanupResultSuccess || result == QiniuRemoteCleanupResultIdempotentSuccess
}

func MarkQiniuKeyTaskSkipped(id int, reason string) error {
	now := common.GetTimestamp()
	return DB.Model(&QiniuKeySyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          QiniuKeyTaskStatusSkipped,
		"last_error":      reason,
		"completed_time":  now,
		"next_retry_time": int64(0),
		"updated_time":    now,
	}).Error
}

func MarkQiniuKeyTaskFailed(id int, retryIntervalSeconds int, err error) error {
	now := common.GetTimestamp()
	nextRetryTime := now + int64(retryIntervalSeconds)
	message := ""
	if err != nil {
		message = err.Error()
	}
	return DB.Model(&QiniuKeySyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          QiniuKeyTaskStatusFailed,
		"retry_count":     gorm.Expr("retry_count + ?", 1),
		"next_retry_time": nextRetryTime,
		"last_error":      message,
		"completed_time":  now,
		"updated_time":    now,
	}).Error
}

func ListRetryableQiniuKeySyncTasks(limit int, runningStaleBefore int64) ([]*QiniuKeySyncTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var tasks []*QiniuKeySyncTask
	now := time.Now().Unix()
	err := DB.Where(
		"status = ? OR (status = ? AND next_retry_time <= ?) OR (status = ? AND started_time <= ?)",
		QiniuKeyTaskStatusPending,
		QiniuKeyTaskStatusFailed,
		now,
		QiniuKeyTaskStatusRunning,
		runningStaleBefore,
	).
		Order("next_retry_time asc, id asc").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func ListUserQiniuKeySyncTasks(userId int, taskType string, status string, limit int) ([]*QiniuKeySyncTask, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	query := DB.Where("user_id = ?", userId)
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var tasks []*QiniuKeySyncTask
	err := query.Order("id desc").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func ListQiniuKeySyncTasks(userId int, taskType string, status string, pageInfo *common.PageInfo) (tasks []*QiniuKeySyncTask, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuKeySyncTask{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
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
		Find(&tasks).Error
	return tasks, total, err
}
