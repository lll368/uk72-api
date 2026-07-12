package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	QiniuChildAccountTaskTypeCreate   = "qiniu_child_account_create"
	QiniuChildAccountTaskTypeFetchKey = "qiniu_child_account_fetch_key"
	QiniuChildAccountTaskTypeDisable  = "qiniu_child_account_disable"
	QiniuChildAccountTaskTypeEnable   = "qiniu_child_account_enable"

	QiniuChildAccountTaskStatusPending = "pending"
	QiniuChildAccountTaskStatusRunning = "running"
	QiniuChildAccountTaskStatusSuccess = "success"
	QiniuChildAccountTaskStatusFailed  = "failed"
	QiniuChildAccountTaskStatusSkipped = "skipped"
)

var qiniuChildAccountBlockingTaskStatuses = []string{
	QiniuChildAccountTaskStatusPending,
	QiniuChildAccountTaskStatusRunning,
}

// QiniuChildAccountSyncTask 记录七牛 OEM 子账户创建、取密钥、启用和禁用补偿任务。
type QiniuChildAccountSyncTask struct {
	Id            int    `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	AccountId     int    `json:"account_id" gorm:"index:idx_qiniu_child_task_account_type_status"`
	TaskType      string `json:"task_type" gorm:"type:varchar(64);index:idx_qiniu_child_task_account_type_status"`
	Status        string `json:"status" gorm:"type:varchar(32);index:idx_qiniu_child_task_account_type_status"`
	RetryCount    int    `json:"retry_count"`
	NextRetryTime int64  `json:"next_retry_time" gorm:"bigint;index"`
	LastError     string `json:"last_error" gorm:"type:text"`
	Payload       string `json:"payload" gorm:"type:text"`
	CreatedBy     int    `json:"created_by" gorm:"index"`
	CreatedTime   int64  `json:"created_time" gorm:"bigint;index"`
	UpdatedTime   int64  `json:"updated_time" gorm:"bigint"`
	StartedTime   int64  `json:"started_time" gorm:"bigint"`
	CompletedTime int64  `json:"completed_time" gorm:"bigint"`
}

func (task *QiniuChildAccountSyncTask) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if task.CreatedTime == 0 {
		task.CreatedTime = now
	}
	if task.UpdatedTime == 0 {
		task.UpdatedTime = now
	}
	if task.Status == "" {
		task.Status = QiniuChildAccountTaskStatusPending
	}
	return nil
}

func (task *QiniuChildAccountSyncTask) BeforeUpdate(tx *gorm.DB) error {
	task.UpdatedTime = common.GetTimestamp()
	return nil
}

func CreateQiniuChildAccountSyncTask(task *QiniuChildAccountSyncTask) error {
	if task == nil {
		return errors.New("七牛子账户同步任务不能为空")
	}
	return DB.Create(task).Error
}

func GetQiniuChildAccountSyncTaskById(id int) (*QiniuChildAccountSyncTask, error) {
	var task QiniuChildAccountSyncTask
	if err := DB.First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func TryMarkQiniuChildAccountTaskRunning(id int, runningStaleBefore int64) (bool, error) {
	now := common.GetTimestamp()
	result := DB.Model(&QiniuChildAccountSyncTask{}).
		Where(
			"id = ? AND (status IN ? OR (status = ? AND started_time <= ?))",
			id,
			[]string{QiniuChildAccountTaskStatusPending, QiniuChildAccountTaskStatusFailed},
			QiniuChildAccountTaskStatusRunning,
			runningStaleBefore,
		).
		Updates(map[string]interface{}{
			"status":       QiniuChildAccountTaskStatusRunning,
			"started_time": now,
			"updated_time": now,
		})
	return result.RowsAffected > 0, result.Error
}

func MarkQiniuChildAccountTaskSuccess(id int) error {
	now := common.GetTimestamp()
	return DB.Model(&QiniuChildAccountSyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          QiniuChildAccountTaskStatusSuccess,
		"last_error":      "",
		"completed_time":  now,
		"next_retry_time": int64(0),
		"updated_time":    now,
	}).Error
}

func MarkQiniuChildAccountTaskFailed(id int, retryIntervalSeconds int, err error) error {
	now := common.GetTimestamp()
	message := ""
	if err != nil {
		message = err.Error()
	}
	return DB.Model(&QiniuChildAccountSyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          QiniuChildAccountTaskStatusFailed,
		"retry_count":     gorm.Expr("retry_count + ?", 1),
		"next_retry_time": now + int64(retryIntervalSeconds),
		"last_error":      message,
		"completed_time":  now,
		"updated_time":    now,
	}).Error
}

func MarkQiniuChildAccountTaskSkipped(id int, reason string) error {
	now := common.GetTimestamp()
	return DB.Model(&QiniuChildAccountSyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          QiniuChildAccountTaskStatusSkipped,
		"last_error":      reason,
		"completed_time":  now,
		"next_retry_time": int64(0),
		"updated_time":    now,
	}).Error
}

func HasBlockingQiniuChildAccountTaskWithTx(tx *gorm.DB, accountId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&QiniuChildAccountSyncTask{}).
		Where("account_id = ? AND status IN ?", accountId, qiniuChildAccountBlockingTaskStatuses).
		Count(&count).Error
	return count > 0, err
}

func ListRetryableQiniuChildAccountSyncTasks(limit int, runningStaleBefore int64) ([]*QiniuChildAccountSyncTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var tasks []*QiniuChildAccountSyncTask
	now := time.Now().Unix()
	err := DB.Where(
		"status = ? OR (status = ? AND next_retry_time <= ?) OR (status = ? AND started_time <= ?)",
		QiniuChildAccountTaskStatusPending,
		QiniuChildAccountTaskStatusFailed,
		now,
		QiniuChildAccountTaskStatusRunning,
		runningStaleBefore,
	).
		Order("next_retry_time asc, id asc").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

type QiniuChildAccountTaskQuery struct {
	AccountId int
	TaskType  string
	Status    string
}

type QiniuChildAccountTaskPage struct {
	Items []*QiniuChildAccountSyncTask
	Total int64
}

func ListQiniuChildAccountSyncTasks(filter QiniuChildAccountTaskQuery, pageInfo *common.PageInfo) (*QiniuChildAccountTaskPage, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuChildAccountSyncTask{})
	if filter.AccountId > 0 {
		query = query.Where("account_id = ?", filter.AccountId)
	}
	if filter.TaskType != "" {
		query = query.Where("task_type = ?", filter.TaskType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var tasks []*QiniuChildAccountSyncTask
	if err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return &QiniuChildAccountTaskPage{Items: tasks, Total: total}, nil
}
