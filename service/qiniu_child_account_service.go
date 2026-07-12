package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	qiniuChildAccountTaskRequestTimeout = 30 * time.Second
)

var (
	qiniuChildAccountTaskAsyncDisabled atomic.Bool
	qiniuChildAccountTaskRetryOnce     sync.Once
	qiniuChildAccountTaskRetryRunning  atomic.Bool
)

type QiniuChildAccountTaskScanResult struct {
	ProcessedCount int      `json:"processed_count"`
	SuccessCount   int      `json:"success_count"`
	SkippedCount   int      `json:"skipped_count"`
	Errors         []string `json:"errors"`
}

type qiniuChildAccountOperationPayload struct {
	Reason string `json:"reason,omitempty"`
}

func disableQiniuChildAccountAsyncForTest(t interface{ Cleanup(func()) }) {
	qiniuChildAccountTaskAsyncDisabled.Store(true)
	t.Cleanup(func() {
		qiniuChildAccountTaskAsyncDisabled.Store(false)
	})
}

func qiniuChildAccountRetryIntervalSeconds() int {
	interval := operation_setting.GetQiniuKeySetting().ChildAccountRetryIntervalSeconds
	if interval <= 0 {
		return operation_setting.QiniuChildAccountDefaultRetryInterval
	}
	return interval
}

func newQiniuChildAccountClient(setting *operation_setting.QiniuKeySetting) (*qiniuKeyClient, error) {
	if err := operation_setting.ValidateQiniuChildAccountSettingForCreate(setting); err != nil {
		return nil, err
	}
	requestTimeout := setting.ChildAccountRequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = setting.RequestTimeout
	}
	if requestTimeout <= 0 {
		requestTimeout = operation_setting.QiniuChildAccountDefaultRequestTimeout
	}
	return &qiniuKeyClient{
		setting: *setting,
		httpClient: &http.Client{
			Timeout: time.Duration(requestTimeout) * time.Second,
		},
	}, nil
}

func CreateQiniuChildAccount(ctx context.Context, operatorId int) (*model.QiniuChildAccount, *model.QiniuChildAccountSyncTask, error) {
	setting := operation_setting.GetQiniuKeySetting()
	if err := operation_setting.ValidateQiniuChildAccountSettingForCreate(setting); err != nil {
		return nil, nil, err
	}
	password, err := common.GenerateRandomCharsKey(setting.ChildAccountPasswordLength)
	if err != nil {
		return nil, nil, err
	}
	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, setting.ChildAccountEmailPrefix, setting.ChildAccountEmailDomain, password)
	if err != nil {
		return nil, nil, err
	}
	if operatorId > 0 {
		_ = model.DB.Model(&model.QiniuChildAccount{}).Where("id = ?", account.Id).Update("created_by", operatorId).Error
		account.CreatedBy = operatorId
	}
	task := &model.QiniuChildAccountSyncTask{
		AccountId: account.Id,
		TaskType:  model.QiniuChildAccountTaskTypeCreate,
		Status:    model.QiniuChildAccountTaskStatusPending,
		CreatedBy: operatorId,
	}
	if err := model.CreateQiniuChildAccountSyncTask(task); err != nil {
		return nil, nil, err
	}
	runQiniuChildAccountTaskAsync(task.Id)
	return account, task, nil
}

func DisableQiniuChildAccount(ctx context.Context, accountId int, operatorId int, reason string) (*model.QiniuChildAccount, *model.QiniuChildAccountSyncTask, error) {
	reason = strings.TrimSpace(reason)
	if accountId <= 0 {
		return nil, nil, errors.New("子账户 ID 无效")
	}
	if reason == "" {
		return nil, nil, errors.New("禁用原因不能为空")
	}
	return enqueueQiniuChildAccountOperationTask(accountId, operatorId, model.QiniuChildAccountTaskTypeDisable, qiniuChildAccountOperationPayload{Reason: reason})
}

func EnableQiniuChildAccount(ctx context.Context, accountId int, operatorId int) (*model.QiniuChildAccount, *model.QiniuChildAccountSyncTask, error) {
	if accountId <= 0 {
		return nil, nil, errors.New("子账户 ID 无效")
	}
	return enqueueQiniuChildAccountOperationTask(accountId, operatorId, model.QiniuChildAccountTaskTypeEnable, qiniuChildAccountOperationPayload{})
}

func enqueueQiniuChildAccountOperationTask(accountId int, operatorId int, taskType string, payload qiniuChildAccountOperationPayload) (*model.QiniuChildAccount, *model.QiniuChildAccountSyncTask, error) {
	var account model.QiniuChildAccount
	var task *model.QiniuChildAccountSyncTask
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&account, "id = ?", accountId).Error; err != nil {
			return err
		}
		blocked, err := model.HasBlockingQiniuChildAccountTaskWithTx(tx, accountId)
		if err != nil {
			return err
		}
		if blocked {
			return errors.New("子账户已有任务执行中，请稍后再试")
		}
		payloadBytes, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		created := &model.QiniuChildAccountSyncTask{
			AccountId: accountId,
			TaskType:  taskType,
			Status:    model.QiniuChildAccountTaskStatusPending,
			Payload:   string(payloadBytes),
			CreatedBy: operatorId,
		}
		if err := tx.Create(created).Error; err != nil {
			return err
		}
		task = created
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	runQiniuChildAccountTaskAsync(task.Id)
	return &account, task, nil
}

func runQiniuChildAccountTaskAsync(taskId int) {
	if qiniuChildAccountTaskAsyncDisabled.Load() {
		return
	}
	gopool.Go(func() {
		if err := ExecuteQiniuChildAccountTask(context.Background(), taskId); err != nil {
			common.SysLog(fmt.Sprintf("qiniu child account task failed task_id=%d error=%s", taskId, SanitizeQiniuChildAccountSecret(err.Error())))
		}
	})
}

func ExecuteQiniuChildAccountTask(ctx context.Context, taskId int) error {
	acquired, err := model.TryMarkQiniuChildAccountTaskRunning(taskId, qiniuChildAccountRunningStaleBefore())
	if err != nil {
		return err
	}
	if !acquired {
		return errors.New("子账户任务正在执行或状态不可重试")
	}
	task, err := model.GetQiniuChildAccountSyncTaskById(taskId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, qiniuChildAccountTaskRequestTimeout)
	defer cancel()
	switch task.TaskType {
	case model.QiniuChildAccountTaskTypeCreate, model.QiniuChildAccountTaskTypeFetchKey:
		err = executeQiniuChildAccountCreateTask(ctx, task)
	case model.QiniuChildAccountTaskTypeDisable:
		err = executeQiniuChildAccountDisableTask(ctx, task)
	case model.QiniuChildAccountTaskTypeEnable:
		err = executeQiniuChildAccountEnableTask(ctx, task)
	default:
		err := fmt.Errorf("未知子账户任务类型: %s", task.TaskType)
		_ = model.MarkQiniuChildAccountTaskFailed(task.Id, qiniuChildAccountRetryIntervalSeconds(), err)
		return err
	}
	return err
}

func qiniuChildAccountRunningStaleBefore() int64 {
	return time.Now().Add(-2 * qiniuChildAccountTaskRequestTimeout).Unix()
}

func executeQiniuChildAccountCreateTask(ctx context.Context, task *model.QiniuChildAccountSyncTask) error {
	account, err := model.GetQiniuChildAccountById(task.AccountId)
	if err != nil {
		return failQiniuChildAccountTask(task, err, true)
	}
	unlock, ok := tryAcquireQiniuTaskLock(fmt.Sprintf("child-account:%d", account.Id))
	if !ok {
		err := errors.New("同一子账户已有任务执行中")
		_ = model.MarkQiniuChildAccountTaskFailed(task.Id, qiniuChildAccountRetryIntervalSeconds(), err)
		return err
	}
	defer unlock()

	client, err := newQiniuChildAccountClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return failQiniuChildAccountTask(task, err, true)
	}
	if strings.TrimSpace(account.UID) == "" {
		loginPassword, err := model.RevealQiniuChildAccountLoginPassword(account.LoginPassword)
		if err != nil {
			if strings.TrimSpace(account.Email) == "" {
				return failQiniuChildAccountTask(task, err, true)
			}
			// 远端已创建但本地补偿失败时，登录密码可能已被清空；此时只能按邮箱幂等查询密钥恢复本地凭据。
		} else {
			remote, err := client.CreateChildAccount(ctx, account.Email, loginPassword)
			if err != nil {
				if !isQiniuChildAccountAlreadyExistsError(err) {
					return failQiniuChildAccountTask(task, err, true)
				}
				remote = &qiniuChildAccountRemote{Email: account.Email}
			}
			if err := model.MarkQiniuChildAccountRemoteCreated(account.Id, remote.UserID, remote.UID, remote.ParentUID, remote.IsDisabled); err != nil {
				return failQiniuChildAccountTask(task, err, true)
			}
			account.UID = remote.UID
			account.RemoteUserID = remote.UserID
			account.ParentUID = remote.ParentUID
		}
	}
	keys, err := client.GetChildKey(ctx, account.UID, account.Email)
	if err != nil {
		return failQiniuChildAccountTask(task, err, true)
	}
	if err := model.MarkQiniuChildAccountCredentials(account.Id, keys.AccessKey, keys.SecretKey, keys.State, keys.BackupAccessKey, keys.BackupSecretKey, keys.BackupState); err != nil {
		return failQiniuChildAccountTask(task, err, true)
	}
	if err := model.MarkQiniuChildAccountStatus(account.Id, model.QiniuChildAccountStatusEnabled, ""); err != nil {
		return failQiniuChildAccountTask(task, err, true)
	}
	return model.MarkQiniuChildAccountTaskSuccess(task.Id)
}

func RetryDueQiniuChildAccountTasks(limit int) (*QiniuChildAccountTaskScanResult, error) {
	tasks, err := model.ListRetryableQiniuChildAccountSyncTasks(limit, qiniuChildAccountRunningStaleBefore())
	if err != nil {
		return nil, err
	}
	result := &QiniuChildAccountTaskScanResult{Errors: make([]string, 0)}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		result.ProcessedCount++
		if err := ExecuteQiniuChildAccountTask(context.Background(), task.Id); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("task_id=%d error=%s", task.Id, SanitizeQiniuChildAccountSecret(err.Error())))
			continue
		}
		reloadedTask, err := model.GetQiniuChildAccountSyncTaskById(task.Id)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("task_id=%d error=%s", task.Id, SanitizeQiniuChildAccountSecret(err.Error())))
			continue
		}
		switch reloadedTask.Status {
		case model.QiniuChildAccountTaskStatusSuccess:
			result.SuccessCount++
		case model.QiniuChildAccountTaskStatusSkipped:
			result.SkippedCount++
		}
	}
	return result, nil
}

func StartQiniuChildAccountTaskRetryTask() {
	qiniuChildAccountTaskRetryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu child account task retry task started: tick=%s", qiniuTaskRetryTickInterval))
			ticker := time.NewTicker(qiniuTaskRetryTickInterval)
			defer ticker.Stop()
			runQiniuChildAccountTaskRetryOnce()
			for range ticker.C {
				runQiniuChildAccountTaskRetryOnce()
			}
		})
	})
}

func runQiniuChildAccountTaskRetryOnce() {
	if !IsQiniuKeyLifecycleEnabled() {
		return
	}
	if !qiniuChildAccountTaskRetryRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuChildAccountTaskRetryRunning.Store(false)
	result, err := RetryDueQiniuChildAccountTasks(qiniuTaskRetryBatchSize)
	if err != nil {
		common.SysLog("qiniu child account retry task scan failed: " + err.Error())
		return
	}
	if result.ProcessedCount > 0 || len(result.Errors) > 0 {
		common.SysLog(fmt.Sprintf(
			"qiniu child account retry task scanned processed=%d success=%d skipped=%d errors=%d",
			result.ProcessedCount,
			result.SuccessCount,
			result.SkippedCount,
			len(result.Errors),
		))
	}
}

func isQiniuChildAccountAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "email_exist") ||
		strings.Contains(message, "email") && strings.Contains(message, "exist")
}

func executeQiniuChildAccountDisableTask(ctx context.Context, task *model.QiniuChildAccountSyncTask) error {
	account, err := model.GetQiniuChildAccountById(task.AccountId)
	if err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if strings.TrimSpace(account.UID) == "" {
		return failQiniuChildAccountTask(task, errors.New("子账户缺少远端 uid"), false)
	}
	payload := qiniuChildAccountOperationPayload{}
	if task.Payload != "" {
		_ = common.UnmarshalJsonStr(task.Payload, &payload)
	}
	unlock, ok := tryAcquireQiniuTaskLock(fmt.Sprintf("child-account:%d", account.Id))
	if !ok {
		err := errors.New("同一子账户已有任务执行中")
		_ = model.MarkQiniuChildAccountTaskFailed(task.Id, qiniuChildAccountRetryIntervalSeconds(), err)
		return err
	}
	defer unlock()
	client, err := newQiniuChildAccountClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if err := client.DisableChildAccount(ctx, account.UID, payload.Reason); err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if err := model.DB.Model(&model.QiniuChildAccount{}).Where("id = ?", account.Id).Updates(map[string]interface{}{
		"status":          model.QiniuChildAccountStatusDisabled,
		"disabled_by":     task.CreatedBy,
		"disabled_reason": strings.TrimSpace(payload.Reason),
		"disabled_time":   common.GetTimestamp(),
		"last_error":      "",
		"updated_time":    common.GetTimestamp(),
	}).Error; err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if err := disableEnabledQiniuTokensForChildAccount(account.Id); err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	return model.MarkQiniuChildAccountTaskSuccess(task.Id)
}

func executeQiniuChildAccountEnableTask(ctx context.Context, task *model.QiniuChildAccountSyncTask) error {
	account, err := model.GetQiniuChildAccountById(task.AccountId)
	if err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if strings.TrimSpace(account.UID) == "" {
		return failQiniuChildAccountTask(task, errors.New("子账户缺少远端 uid"), false)
	}
	unlock, ok := tryAcquireQiniuTaskLock(fmt.Sprintf("child-account:%d", account.Id))
	if !ok {
		err := errors.New("同一子账户已有任务执行中")
		_ = model.MarkQiniuChildAccountTaskFailed(task.Id, qiniuChildAccountRetryIntervalSeconds(), err)
		return err
	}
	defer unlock()
	client, err := newQiniuChildAccountClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if err := client.EnableChildAccount(ctx, account.UID); err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	if err := model.DB.Model(&model.QiniuChildAccount{}).Where("id = ?", account.Id).Updates(map[string]interface{}{
		"status":          model.QiniuChildAccountStatusEnabled,
		"disabled_reason": "",
		"disabled_time":   int64(0),
		"last_error":      "",
		"updated_time":    common.GetTimestamp(),
	}).Error; err != nil {
		return failQiniuChildAccountTask(task, err, false)
	}
	return model.MarkQiniuChildAccountTaskSuccess(task.Id)
}

func RetryQiniuChildAccountTaskById(taskId int) error {
	if taskId <= 0 {
		return errors.New("任务 ID 无效")
	}
	task, err := model.GetQiniuChildAccountSyncTaskById(taskId)
	if err != nil {
		return err
	}
	if task.Status != model.QiniuChildAccountTaskStatusFailed {
		return errors.New("当前子账户任务不可重试")
	}
	return ExecuteQiniuChildAccountTask(context.Background(), taskId)
}

func disableEnabledQiniuTokensForChildAccount(accountId int) error {
	if accountId <= 0 {
		return nil
	}
	var tokens []model.Token
	if err := model.DB.
		Select("id", "user_id", "key").
		Where("provider = ? AND qiniu_child_account_id = ? AND status = ?", model.TokenProviderQiniu, accountId, common.TokenStatusEnabled).
		Find(&tokens).Error; err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	if err := model.DB.Model(&model.Token{}).
		Where("provider = ? AND qiniu_child_account_id = ? AND status = ?", model.TokenProviderQiniu, accountId, common.TokenStatusEnabled).
		Update("status", common.TokenStatusDisabled).Error; err != nil {
		return err
	}
	if common.RedisEnabled {
		userIds := make(map[int]bool, len(tokens))
		for _, token := range tokens {
			if token.UserId > 0 {
				userIds[token.UserId] = true
			}
		}
		for userId := range userIds {
			currentUserId := userId
			gopool.Go(func() {
				if err := model.InvalidateUserTokensCache(currentUserId); err != nil {
					common.SysLog(fmt.Sprintf("failed to invalidate user token cache user_id=%d error=%s", currentUserId, err.Error()))
				}
			})
		}
	}
	return nil
}

func failQiniuChildAccountTask(task *model.QiniuChildAccountSyncTask, err error, markAccountFailed bool) error {
	safeErr := errors.New(SanitizeQiniuChildAccountSecret(sanitizeQiniuTaskError(err)))
	_ = model.MarkQiniuChildAccountTaskFailed(task.Id, qiniuChildAccountRetryIntervalSeconds(), safeErr)
	if markAccountFailed && task.AccountId > 0 {
		_ = model.MarkQiniuChildAccountStatus(task.AccountId, model.QiniuChildAccountStatusFailed, safeErr.Error())
	}
	return safeErr
}
