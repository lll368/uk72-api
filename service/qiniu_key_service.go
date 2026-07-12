package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	qiniuTaskRequestTimeout    = 30 * time.Second
	qiniuTaskRetryTickInterval = 1 * time.Minute
	qiniuTaskRetryBatchSize    = 100
	qiniuTaskErrorMaxLength    = 512

	qiniuKeyInitialLimitZero    = "zero"
	qiniuKeyInitialLimitBalance = "balance"
	qiniuKeyNameMaxRunes        = 20
	qiniuDefaultTokenGroup      = "default"

	qiniuLegacyTokensRevokedOptionKey = "qiniu_key_legacy_tokens_revoked"
)

var qiniuTaskLocks sync.Map

var (
	qiniuTaskRetryOnce     sync.Once
	qiniuTaskRetryRunning  atomic.Bool
	qiniuTaskAsyncDisabled atomic.Bool
)

var qiniuSensitiveKeyPattern = regexp.MustCompile(`sk-[0-9a-fA-F]{64}`)

type QiniuKeyStatus struct {
	Enabled            bool   `json:"enabled"`
	HasKey             bool   `json:"has_key"`
	TaskStatus         string `json:"task_status"`
	TaskRetryable      bool   `json:"task_retryable"`
	LastError          string `json:"last_error"`
	NextRetryTime      int64  `json:"next_retry_time"`
	CanCreateToken     bool   `json:"can_create_token"`
	RevokeBlocked      bool   `json:"revoke_blocked"`
	BlockingTaskType   string `json:"blocking_task_type"`
	BlockingTaskStatus string `json:"blocking_task_status"`
}

type QiniuKeyTaskScanResult struct {
	ProcessedCount int      `json:"processed_count"`
	SuccessCount   int      `json:"success_count"`
	SkippedCount   int      `json:"skipped_count"`
	Errors         []string `json:"errors"`
}

type qiniuKeyCreateTaskPayload struct {
	Name                string  `json:"name"`
	ExpiredTime         int64   `json:"expired_time"`
	ModelLimitsEnabled  bool    `json:"model_limits_enabled"`
	ModelLimits         string  `json:"model_limits"`
	AllowIps            *string `json:"allow_ips"`
	Group               string  `json:"group"`
	CrossGroupRetry     bool    `json:"cross_group_retry"`
	InitialLimitMode    string  `json:"initial_limit_mode"`
	QiniuChildAccountId int     `json:"qiniu_child_account_id"`
}

type qiniuKeyRevokeTaskPayload struct {
	CreatedTime int64 `json:"created_time"`
}

func IsQiniuKeyLifecycleEnabled() bool {
	return operation_setting.GetQiniuKeySetting().Enabled
}

func IsQiniuManagedTokenKey(key string) bool {
	return isQiniuAPIKeyBody(key)
}

func IsQiniuManagedToken(token *model.Token) bool {
	return token != nil && token.IsQiniuManaged()
}

func qiniuRetryIntervalSeconds() int {
	interval := operation_setting.GetQiniuKeySetting().RetryIntervalSeconds
	if interval <= 0 {
		return operation_setting.QiniuKeyDefaultRetryInterval
	}
	return interval
}

func qiniuDefaultKeyName(userId int) string {
	return fmt.Sprintf("dk-%d", userId)
}

// EnqueueDefaultQiniuKeyCreateTask 在用户注册成功后创建默认 Key 补偿任务。
func EnqueueDefaultQiniuKeyCreateTask(userId int, username string) error {
	if userId <= 0 || !IsQiniuKeyLifecycleEnabled() {
		return nil
	}
	task, err := enqueueQiniuCreateTask(userId, model.QiniuKeyTaskTypeDefaultCreate, qiniuKeyCreateTaskPayload{
		Name:             qiniuDefaultKeyName(userId),
		ExpiredTime:      -1,
		Group:            qiniuDefaultTokenGroup,
		InitialLimitMode: qiniuKeyInitialLimitBalance,
	}, false)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}
	common.SysLog(fmt.Sprintf("qiniu default key task created user_id=%d task_id=%d", userId, task.Id))
	runQiniuKeyTaskAsync(task.Id)
	return nil
}

func enqueueQiniuCreateTask(userId int, taskType string, payload qiniuKeyCreateTaskPayload, returnBlockedError bool) (*model.QiniuKeySyncTask, error) {
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	name, err := normalizeQiniuKeyName(payload.Name)
	if err != nil {
		return nil, err
	}
	payload.Name = name
	if payload.InitialLimitMode == "" {
		payload.InitialLimitMode = qiniuKeyInitialLimitZero
	}
	var task *model.QiniuKeySyncTask
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockQiniuUserTx(tx, userId); err != nil {
			return err
		}
		count, err := model.CountUserTokensWithTx(tx, userId)
		if err != nil {
			return err
		}
		if count >= 1 {
			if returnBlockedError {
				return errors.New("当前用户最多只能创建 1 个 Key")
			}
			return nil
		}
		revokeBlocked, err := model.HasBlockingQiniuRevokeTaskWithTx(tx, userId)
		if err != nil {
			return err
		}
		if revokeBlocked {
			if returnBlockedError {
				return errors.New("旧 Key 远端禁用任务未完成，请稍后再创建新 Key")
			}
			return nil
		}
		blocked, err := model.HasBlockingQiniuCreateTaskWithTx(tx, userId)
		if err != nil {
			return err
		}
		if blocked {
			if returnBlockedError {
				return errors.New("默认 Key 正在创建或等待补偿，请稍后再试")
			}
			return nil
		}
		payloadBytes, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		created := &model.QiniuKeySyncTask{
			TaskType: taskType,
			UserId:   userId,
			Status:   model.QiniuKeyTaskStatusPending,
			Payload:  string(payloadBytes),
		}
		if err := tx.Create(created).Error; err != nil {
			return err
		}
		task = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

func normalizeQiniuKeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "默认 Key", nil
	}
	if len([]rune(name)) > qiniuKeyNameMaxRunes {
		return "", fmt.Errorf("Key 名称不能超过 %d 个字符", qiniuKeyNameMaxRunes)
	}
	return name, nil
}

func lockQiniuUserTx(tx *gorm.DB, userId int) error {
	if tx == nil {
		tx = model.DB
	}
	query := tx
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var user model.User
	return query.Select("id").Where("id = ?", userId).First(&user).Error
}

func qiniuInitialQuotaLimit(userId int) (float64, error) {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return 0, err
	}
	limit := quotaToWalletAmount(user.Quota)
	if limit < 0 {
		return 0, nil
	}
	return limit, nil
}

func resolveQiniuCreateTaskAccountIdentity(ctx context.Context, task *model.QiniuKeySyncTask, payload *qiniuKeyCreateTaskPayload) (*QiniuAccountIdentityResolution, error) {
	if task == nil || payload == nil {
		return nil, errors.New("Key 创建任务为空")
	}
	// 老任务可能已经由父账号创建了远端 Key，但还没完成额度初始化；这种任务不能在重试时改判到子账号。
	if strings.TrimSpace(task.QiniuKey) != "" && payload.QiniuChildAccountId == 0 {
		return &QiniuAccountIdentityResolution{
			AccountId:      0,
			AssignmentMode: operation_setting.QiniuChildAccountAssignmentModeParentOnly,
			Source:         QiniuAccountIdentitySourceParent,
			UserId:         task.UserId,
		}, nil
	}
	resolution, err := ResolveQiniuAccountIdentityForNextToken(ctx, task.UserId, payload.QiniuChildAccountId, 0)
	if err != nil {
		var blocker *QiniuAccountIdentityRetryableBlocker
		if errors.As(err, &blocker) && blocker.AccountId > 0 && payload.QiniuChildAccountId != blocker.AccountId {
			payload.QiniuChildAccountId = blocker.AccountId
			if updateErr := persistQiniuCreateTaskPayload(task, payload); updateErr != nil {
				return nil, updateErr
			}
		}
		return nil, err
	}
	if resolution == nil {
		return nil, errors.New("七牛账号身份解析失败")
	}
	if payload.QiniuChildAccountId != resolution.AccountId {
		payload.QiniuChildAccountId = resolution.AccountId
		if err := persistQiniuCreateTaskPayload(task, payload); err != nil {
			return nil, err
		}
	}
	return resolution, nil
}

func persistQiniuCreateTaskPayload(task *model.QiniuKeySyncTask, payload *qiniuKeyCreateTaskPayload) error {
	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	result := model.DB.Model(&model.QiniuKeySyncTask{}).
		Where("id = ?", task.Id).
		Update("payload", string(payloadBytes))
	if result.Error != nil {
		return result.Error
	}
	task.Payload = string(payloadBytes)
	return nil
}

func qiniuHistoricalTokenAccountId(tokenId int) (int, error) {
	if tokenId <= 0 {
		return 0, nil
	}
	var token model.Token
	err := model.DB.Unscoped().
		Select("id", "provider", "qiniu_child_account_id").
		First(&token, "id = ?", tokenId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !token.IsQiniuManaged() {
		return 0, nil
	}
	return token.QiniuChildAccountId, nil
}

// EnqueueQiniuQuotaSyncTask 在充值成功后创建 Key 总额度同步补偿任务。
func EnqueueQiniuQuotaSyncTask(userId int) error {
	if userId <= 0 || !IsQiniuKeyLifecycleEnabled() {
		return nil
	}
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeQuotaSync,
		UserId:   userId,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	token, err := model.GetFirstEnabledUserToken(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog(fmt.Sprintf("qiniu quota sync deferred user_id=%d reason=no_enabled_token", userId))
		} else {
			return err
		}
	} else if !IsQiniuManagedToken(token) {
		common.SysLog(fmt.Sprintf("qiniu quota sync skipped user_id=%d token_id=%d reason=legacy_local_key", userId, token.Id))
		return nil
	} else {
		task.TokenId = token.Id
		task.QiniuKey = token.Key
	}
	if err := model.CreateQiniuKeySyncTask(task); err != nil {
		return err
	}
	common.SysLog(fmt.Sprintf("qiniu quota sync task created user_id=%d token_id=%d task_id=%d key=%s", userId, task.TokenId, task.Id, maskQiniuAPIKey(task.QiniuKey)))
	runQiniuKeyTaskAsync(task.Id)
	return nil
}

func runQiniuKeyTaskAsync(taskId int) {
	if qiniuTaskAsyncDisabled.Load() {
		return
	}
	gopool.Go(func() {
		if err := ExecuteQiniuKeyTask(context.Background(), taskId); err != nil {
			common.SysLog(fmt.Sprintf("qiniu key task failed task_id=%d error=%s", taskId, sanitizeQiniuTaskError(err)))
		}
	})
}

func ExecuteQiniuKeyTask(ctx context.Context, taskId int) error {
	acquired, err := model.TryMarkQiniuKeyTaskRunning(taskId, qiniuRunningStaleBefore())
	if err != nil {
		return err
	}
	if !acquired {
		return errors.New("Key 任务正在执行或状态不可重试")
	}
	task, err := model.GetQiniuKeySyncTaskById(taskId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, qiniuTaskRequestTimeout)
	defer cancel()
	switch task.TaskType {
	case model.QiniuKeyTaskTypeDefaultCreate:
		err = executeQiniuCreateKeyTask(ctx, task, true)
	case model.QiniuKeyTaskTypeManualCreate:
		err = executeQiniuCreateKeyTask(ctx, task, false)
	case model.QiniuKeyTaskTypeQuotaSync:
		err = executeQiniuQuotaSyncTask(ctx, task)
	case model.QiniuKeyTaskTypeRevoke:
		err = executeQiniuRevokeTask(ctx, task)
	default:
		err := fmt.Errorf("未知 Key 任务类型: %s", task.TaskType)
		_ = model.MarkQiniuKeyTaskFailed(task.Id, qiniuRetryIntervalSeconds(), err)
		return err
	}
	if err != nil {
		logQiniuTaskFailure(task.Id, err)
	}
	return err
}

func qiniuRunningStaleBefore() int64 {
	return time.Now().Add(-2 * qiniuTaskRequestTimeout).Unix()
}

func executeQiniuCreateKeyTask(ctx context.Context, task *model.QiniuKeySyncTask, defaultTask bool) error {
	if task == nil {
		return errors.New("Key 创建任务为空")
	}
	unlock, ok := tryAcquireQiniuTaskLock(fmt.Sprintf("create:%d", task.UserId))
	if !ok {
		err := errors.New("同一用户已有 Key 创建任务执行中")
		_ = model.MarkQiniuKeyTaskFailed(task.Id, qiniuRetryIntervalSeconds(), err)
		return err
	}
	defer unlock()

	count, err := model.CountUserTokens(task.UserId)
	if err != nil {
		return failQiniuTask(task.Id, err)
	}
	if count > 0 {
		return model.MarkQiniuKeyTaskSkipped(task.Id, "用户已存在 Key")
	}

	payload := qiniuKeyCreateTaskPayload{
		Name:             qiniuDefaultKeyName(task.UserId),
		ExpiredTime:      -1,
		Group:            qiniuDefaultTokenGroup,
		InitialLimitMode: qiniuKeyInitialLimitZero,
	}
	if !defaultTask {
		payload.InitialLimitMode = qiniuKeyInitialLimitBalance
	}
	if task.Payload != "" {
		if err := common.UnmarshalJsonStr(task.Payload, &payload); err != nil {
			return failQiniuTask(task.Id, err)
		}
	}
	if defaultTask {
		name := strings.TrimSpace(payload.Name)
		if name == "" || name == "默认 Key" {
			// 兼容已经入库但尚未创建远端 Key 的旧默认任务，避免七牛侧默认 Key 名称重复。
			payload.Name = qiniuDefaultKeyName(task.UserId)
		}
	}
	payload.Name, err = normalizeQiniuKeyName(payload.Name)
	if err != nil {
		return failQiniuTask(task.Id, err)
	}
	if payload.ExpiredTime == 0 {
		payload.ExpiredTime = -1
	}
	if payload.InitialLimitMode == "" {
		if defaultTask {
			payload.InitialLimitMode = qiniuKeyInitialLimitZero
		} else {
			payload.InitialLimitMode = qiniuKeyInitialLimitBalance
		}
	}
	if strings.TrimSpace(payload.Group) == "" {
		// 七牛托管 Key 固定落在 default 分组，避免受“自动分组”后台开关影响。
		payload.Group = qiniuDefaultTokenGroup
	}
	if payload.Group != "auto" {
		payload.CrossGroupRetry = false
	}

	accountIdentity, err := resolveQiniuCreateTaskAccountIdentity(ctx, task, &payload)
	if err != nil {
		return failQiniuTask(task.Id, err)
	}
	client, err := NewQiniuAccountIdentityClient(accountIdentity.AccountId, QiniuAccountOperationCreate)
	if err != nil {
		return failQiniuTask(task.Id, err)
	}
	keyBody := task.QiniuKey
	if keyBody != "" {
		if _, err := normalizeQiniuAPIKey(keyBody); err != nil {
			return failQiniuTask(task.Id, err)
		}
	} else {
		var err error
		keyBody, err = client.CreateAPIKey(ctx, payload.Name)
		if err != nil {
			return failQiniuTask(task.Id, err)
		}
		if err := model.MarkQiniuKeyTaskRemoteKey(task.Id, keyBody); err != nil {
			return failQiniuTaskWithKey(task.Id, 0, keyBody, err)
		}
		task.QiniuKey = keyBody
	}

	targetLimit, err := qiniuInitialQuotaLimit(task.UserId)
	if err != nil {
		return failQiniuTaskWithKey(task.Id, 0, keyBody, err)
	}
	if err := client.SetAPIKeyTotalQuota(ctx, keyBody, targetLimit); err != nil {
		return failQiniuTaskWithKey(task.Id, 0, keyBody, err)
	}
	token := model.Token{
		UserId:              task.UserId,
		Name:                payload.Name,
		Key:                 keyBody,
		Provider:            model.TokenProviderQiniu,
		CreatedTime:         common.GetTimestamp(),
		AccessedTime:        common.GetTimestamp(),
		ExpiredTime:         payload.ExpiredTime,
		RemainQuota:         0,
		UnlimitedQuota:      true,
		ModelLimitsEnabled:  payload.ModelLimitsEnabled,
		ModelLimits:         payload.ModelLimits,
		AllowIps:            payload.AllowIps,
		Group:               payload.Group,
		CrossGroupRetry:     payload.CrossGroupRetry,
		QiniuChildAccountId: accountIdentity.AccountId,
	}
	if token.Group == "" {
		token.Group = qiniuDefaultTokenGroup
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		return createQiniuInitialQuotaBaselineTx(tx, task.UserId, token.Id, targetLimit)
	}); err != nil {
		return failQiniuTaskWithKey(task.Id, 0, keyBody, err)
	}
	common.SysLog(fmt.Sprintf("qiniu key created user_id=%d token_id=%d task_type=%s key=%s initial_limit=%.6f", task.UserId, token.Id, task.TaskType, maskQiniuAPIKey(keyBody), targetLimit))
	return model.MarkQiniuKeyTaskSuccess(task.Id, token.Id, keyBody)
}

func executeQiniuQuotaSyncTask(ctx context.Context, task *model.QiniuKeySyncTask) error {
	if task == nil {
		return errors.New("额度同步任务为空")
	}
	return model.MarkQiniuKeyTaskSkipped(task.Id, "额度同步已由 quota grant 增量授权替代")
}

func executeQiniuQuotaSyncTaskLegacy(ctx context.Context, task *model.QiniuKeySyncTask) error {
	if task == nil {
		return errors.New("额度同步任务为空")
	}
	tokenId := task.TokenId
	if tokenId <= 0 {
		token, err := model.GetFirstEnabledUserToken(task.UserId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				blocked, blockErr := model.HasBlockingQiniuCreateTask(task.UserId)
				if blockErr != nil {
					return failQiniuTask(task.Id, blockErr)
				}
				if blocked {
					return failQiniuTask(task.Id, errors.New("Key 创建任务未完成，等待后重试"))
				}
				return model.MarkQiniuKeyTaskSkipped(task.Id, "用户没有可用 Key")
			}
			return failQiniuTask(task.Id, err)
		}
		tokenId = token.Id
		task.QiniuKey = token.Key
	}
	token, err := model.GetTokenByIds(tokenId, task.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.MarkQiniuKeyTaskSkipped(task.Id, "Key 已不存在")
		}
		return failQiniuTask(task.Id, err)
	}
	if token.Status != common.TokenStatusEnabled {
		return model.MarkQiniuKeyTaskSkipped(task.Id, "Key 已不可用")
	}
	if !IsQiniuManagedToken(token) {
		return model.MarkQiniuKeyTaskSkipped(task.Id, "非托管 Key 无需同步额度")
	}
	unlock, ok := tryAcquireQiniuTaskLock(qiniuKeyLifecycleTaskLockKey(token.Key))
	if !ok {
		err := errors.New("同一 Key 已有生命周期任务执行中")
		_ = model.MarkQiniuKeyTaskFailed(task.Id, qiniuRetryIntervalSeconds(), err)
		return err
	}
	defer unlock()

	client, err := newQiniuKeyClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return failQiniuTaskWithKey(task.Id, token.Id, token.Key, err)
	}
	usedAmount, err := client.GetAPIKeyUsedAmount(ctx, token.Key, token.CreatedTime)
	if err != nil {
		return failQiniuTaskWithKey(task.Id, token.Id, token.Key, err)
	}
	user, err := model.GetUserById(task.UserId, false)
	if err != nil {
		return failQiniuTaskWithKey(task.Id, token.Id, token.Key, err)
	}
	targetLimit := usedAmount + quotaToWalletAmount(user.Quota)
	if targetLimit < 0 {
		targetLimit = 0
	}
	if err := client.SetAPIKeyTotalQuota(ctx, token.Key, targetLimit); err != nil {
		return failQiniuTaskWithKey(task.Id, token.Id, token.Key, err)
	}
	common.SysLog(fmt.Sprintf("qiniu quota synced user_id=%d token_id=%d key=%s used_amount=%.6f target_limit=%.6f", task.UserId, token.Id, maskQiniuAPIKey(token.Key), usedAmount, targetLimit))
	return model.MarkQiniuKeyTaskSuccess(task.Id, token.Id, token.Key)
}

func executeQiniuRevokeTask(ctx context.Context, task *model.QiniuKeySyncTask) error {
	if task == nil {
		return errors.New("Key 作废任务为空")
	}
	keyBody := strings.TrimSpace(task.QiniuKey)
	if keyBody == "" {
		return model.MarkQiniuKeyTaskSkipped(task.Id, "作废任务缺少 Key")
	}
	if !isQiniuAPIKeyBody(keyBody) {
		return model.MarkQiniuKeyTaskSkipped(task.Id, "非托管 Key 无需远端作废")
	}
	unlock, ok := tryAcquireQiniuTaskLock(qiniuKeyLifecycleTaskLockKey(keyBody))
	if !ok {
		err := errors.New("同一 Key 已有生命周期任务执行中")
		_ = model.MarkQiniuKeyTaskFailed(task.Id, qiniuRetryIntervalSeconds(), err)
		return err
	}
	defer unlock()

	payload := qiniuKeyRevokeTaskPayload{}
	if task.Payload != "" {
		if err := common.UnmarshalJsonStr(task.Payload, &payload); err != nil {
			return failQiniuTaskWithKey(task.Id, task.TokenId, keyBody, err)
		}
	}
	accountId, err := qiniuHistoricalTokenAccountId(task.TokenId)
	if err != nil {
		return failQiniuTaskWithKey(task.Id, task.TokenId, keyBody, err)
	}
	client, err := NewQiniuAccountIdentityClient(accountId, QiniuAccountOperationHistoricalRevoke)
	if err != nil {
		return failQiniuTaskWithKey(task.Id, task.TokenId, keyBody, err)
	}
	fallbackPath := "total_zero"
	err = client.SetAPIKeyTotalQuota(ctx, keyBody, 0)
	if err != nil {
		if !isQiniuTotalQuotaBelowUsedError(err) {
			return failQiniuTaskWithKey(task.Id, task.TokenId, keyBody, err)
		}
		fallbackPath = "daily_zero"
		if dailyErr := client.SetAPIKeyDailyQuotaZero(ctx, keyBody); dailyErr != nil {
			return failQiniuTaskWithKey(task.Id, task.TokenId, keyBody, dailyErr)
		}
	}
	common.SysLog(fmt.Sprintf("qiniu key revoked user_id=%d token_id=%d task_id=%d key=%s fallback_path=%s retry_count=%d", task.UserId, task.TokenId, task.Id, maskQiniuAPIKey(keyBody), fallbackPath, task.RetryCount))
	if err := model.MarkQiniuKeyTaskRemoteCleanupResult(task.Id, model.QiniuRemoteCleanupResultSuccess); err != nil {
		return failQiniuTaskWithKey(task.Id, task.TokenId, keyBody, err)
	}
	return model.MarkQiniuKeyTaskSuccess(task.Id, task.TokenId, keyBody)
}

func CreateQiniuManagedToken(ctx context.Context, userId int, requested model.Token) error {
	if userId <= 0 {
		return errors.New("用户不存在")
	}
	task, err := enqueueQiniuCreateTask(userId, model.QiniuKeyTaskTypeManualCreate, qiniuKeyCreateTaskPayload{
		Name:               requested.Name,
		ExpiredTime:        requested.ExpiredTime,
		ModelLimitsEnabled: requested.ModelLimitsEnabled,
		ModelLimits:        requested.ModelLimits,
		AllowIps:           requested.AllowIps,
		Group:              requested.Group,
		CrossGroupRetry:    requested.CrossGroupRetry,
		InitialLimitMode:   qiniuKeyInitialLimitBalance,
	}, true)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("当前用户最多只能创建 1 个 Key")
	}
	common.SysLog(fmt.Sprintf("qiniu manual key task created user_id=%d task_id=%d", userId, task.Id))
	runQiniuKeyTaskAsync(task.Id)
	return nil
}

func DeleteTokenAndEnqueueQiniuRevoke(userId int, tokenId int) (*model.Token, error) {
	if userId <= 0 || tokenId <= 0 {
		return nil, errors.New("用户或 token 无效")
	}
	var deletedToken model.Token
	var revokeTasks []*model.QiniuKeySyncTask
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", tokenId, userId).First(&deletedToken).Error; err != nil {
			return err
		}
		needsRevoke := shouldCreateQiniuRevokeTask(&deletedToken)
		if needsRevoke {
			if err := lockQiniuUserTx(tx, userId); err != nil {
				return err
			}
		}
		if err := tx.Delete(&deletedToken).Error; err != nil {
			return err
		}
		if needsRevoke {
			task, err := createQiniuKeyRevokeTaskTx(tx, userId, deletedToken.Id, deletedToken.Key, deletedToken.CreatedTime)
			if err != nil {
				return err
			}
			if task != nil {
				revokeTasks = append(revokeTasks, task)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	afterQiniuTokenInvalidated(userId, revokeTasks)
	return &deletedToken, nil
}

func BatchDeleteTokensAndEnqueueQiniuRevoke(userId int, ids []int) (int, error) {
	if userId <= 0 {
		return 0, errors.New("用户不存在")
	}
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空！")
	}
	var tokens []model.Token
	var revokeTasks []*model.QiniuKeySyncTask
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND id IN ?", userId, ids).Find(&tokens).Error; err != nil {
			return err
		}
		needsUserLock := false
		for i := range tokens {
			if shouldCreateQiniuRevokeTask(&tokens[i]) {
				needsUserLock = true
				break
			}
		}
		if needsUserLock {
			if err := lockQiniuUserTx(tx, userId); err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ? AND id IN ?", userId, ids).Delete(&model.Token{}).Error; err != nil {
			return err
		}
		for i := range tokens {
			if !shouldCreateQiniuRevokeTask(&tokens[i]) {
				continue
			}
			task, err := createQiniuKeyRevokeTaskTx(tx, userId, tokens[i].Id, tokens[i].Key, tokens[i].CreatedTime)
			if err != nil {
				return err
			}
			if task != nil {
				revokeTasks = append(revokeTasks, task)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	afterQiniuTokenInvalidated(userId, revokeTasks)
	return len(tokens), nil
}

func shouldCreateQiniuRevokeTask(token *model.Token) bool {
	return token != nil && IsQiniuKeyLifecycleEnabled() && IsQiniuManagedToken(token) && isQiniuAPIKeyBody(token.Key)
}

func afterQiniuTokenInvalidated(userId int, revokeTasks []*model.QiniuKeySyncTask) {
	if common.RedisEnabled {
		gopool.Go(func() {
			if err := model.InvalidateUserTokensCache(userId); err != nil {
				common.SysLog(fmt.Sprintf("failed to invalidate user token cache user_id=%d error=%s", userId, err.Error()))
			}
		})
	}
	for _, task := range revokeTasks {
		if task == nil {
			continue
		}
		common.SysLog(fmt.Sprintf("qiniu revoke task created user_id=%d token_id=%d task_id=%d key=%s", task.UserId, task.TokenId, task.Id, maskQiniuAPIKey(task.QiniuKey)))
		runQiniuKeyTaskAsync(task.Id)
	}
}

func EnqueueQiniuKeyRevokeTask(userId int, tokenId int, qiniuKey string, createdTime int64) error {
	task, err := createQiniuKeyRevokeTaskTx(model.DB, userId, tokenId, qiniuKey, createdTime)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}
	common.SysLog(fmt.Sprintf("qiniu revoke task created user_id=%d token_id=%d task_id=%d key=%s", userId, tokenId, task.Id, maskQiniuAPIKey(qiniuKey)))
	runQiniuKeyTaskAsync(task.Id)
	return nil
}

func createQiniuKeyRevokeTaskTx(tx *gorm.DB, userId int, tokenId int, qiniuKey string, createdTime int64) (*model.QiniuKeySyncTask, error) {
	if userId <= 0 || !IsQiniuKeyLifecycleEnabled() {
		return nil, nil
	}
	if !isQiniuAPIKeyBody(qiniuKey) {
		return nil, nil
	}
	if tx == nil {
		tx = model.DB
	}
	payloadBytes, err := common.Marshal(qiniuKeyRevokeTaskPayload{CreatedTime: createdTime})
	if err != nil {
		return nil, err
	}
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeRevoke,
		UserId:   userId,
		TokenId:  tokenId,
		QiniuKey: qiniuKey,
		Status:   model.QiniuKeyTaskStatusPending,
		Payload:  string(payloadBytes),
	}
	if err := tx.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func RevokeLegacyQiniuKeysOnce() error {
	if !IsQiniuKeyLifecycleEnabled() {
		return nil
	}
	if qiniuLegacyTokensRevoked() {
		return nil
	}
	cutoff := common.GetTimestamp()
	for {
		tokens, err := model.ListActiveTokensCreatedBefore(cutoff, 100)
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			break
		}
		for _, token := range tokens {
			if isQiniuAPIKeyBody(token.Key) {
				if err := EnqueueQiniuKeyRevokeTask(token.UserId, token.Id, token.Key, token.CreatedTime); err != nil {
					return err
				}
			}
			if err := token.Delete(); err != nil {
				return err
			}
		}
	}
	option := model.Option{Key: qiniuLegacyTokensRevokedOptionKey}
	if err := model.DB.FirstOrCreate(&option, model.Option{Key: qiniuLegacyTokensRevokedOptionKey}).Error; err != nil {
		return err
	}
	option.Value = "true"
	if err := model.DB.Save(&option).Error; err != nil {
		return err
	}
	common.SysLog("qiniu legacy tokens revoked")
	return nil
}

func qiniuLegacyTokensRevoked() bool {
	var count int64
	if err := model.DB.Model(&model.Option{}).
		Where("key = ? AND value = ?", qiniuLegacyTokensRevokedOptionKey, "true").
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func GetUserQiniuKeyStatus(userId int) (*QiniuKeyStatus, error) {
	status := &QiniuKeyStatus{
		Enabled:        IsQiniuKeyLifecycleEnabled(),
		CanCreateToken: true,
	}
	count, err := model.CountUserTokens(userId)
	if err != nil {
		return nil, err
	}
	status.HasKey = count > 0
	if count >= 1 {
		status.CanCreateToken = false
	}
	if status.Enabled {
		revokeTask, err := model.GetLatestBlockingQiniuRevokeTask(userId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if revokeTask != nil {
			applyQiniuRevokeBlockedStatus(status, revokeTask)
			return status, nil
		}
	}
	if status.Enabled && !status.HasKey {
		if err := EnqueueDefaultQiniuKeyCreateTask(userId, ""); err != nil {
			return nil, err
		}
	}
	task, err := model.GetLatestQiniuCreateTask(userId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return status, nil
	}
	status.TaskStatus = task.Status
	status.TaskRetryable = task.Status == model.QiniuKeyTaskStatusFailed
	if task.Status == model.QiniuKeyTaskStatusFailed {
		status.LastError = "API Key 创建失败，请重试或联系管理员"
	}
	status.NextRetryTime = task.NextRetryTime
	if task.Status == model.QiniuKeyTaskStatusPending || task.Status == model.QiniuKeyTaskStatusRunning || task.Status == model.QiniuKeyTaskStatusFailed {
		status.CanCreateToken = false
		status.BlockingTaskType = task.TaskType
		status.BlockingTaskStatus = task.Status
	}
	return status, nil
}

func applyQiniuRevokeBlockedStatus(status *QiniuKeyStatus, task *model.QiniuKeySyncTask) {
	if status == nil || task == nil {
		return
	}
	status.CanCreateToken = false
	status.RevokeBlocked = true
	status.TaskStatus = task.Status
	status.TaskRetryable = task.Status == model.QiniuKeyTaskStatusFailed
	status.NextRetryTime = task.NextRetryTime
	status.BlockingTaskType = task.TaskType
	status.BlockingTaskStatus = task.Status
	if task.Status == model.QiniuKeyTaskStatusFailed {
		status.LastError = "API Key 远端禁用失败，请等待重试或联系管理员"
	}
}

func RetryQiniuDefaultKeyTask(userId int) error {
	task, err := model.GetLatestQiniuCreateTask(userId)
	if err != nil {
		return err
	}
	if task.Status != model.QiniuKeyTaskStatusFailed {
		return errors.New("当前没有可重试的 Key 创建任务")
	}
	return ExecuteQiniuKeyTask(context.Background(), task.Id)
}

func RetryQiniuKeyTaskById(taskId int) error {
	if taskId <= 0 {
		return errors.New("任务 ID 无效")
	}
	return ExecuteQiniuKeyTask(context.Background(), taskId)
}

func RetryDueQiniuKeyTasks(limit int) (*QiniuKeyTaskScanResult, error) {
	tasks, err := model.ListRetryableQiniuKeySyncTasks(limit, qiniuRunningStaleBefore())
	if err != nil {
		return nil, err
	}
	result := &QiniuKeyTaskScanResult{
		Errors: make([]string, 0),
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		result.ProcessedCount++
		if err := ExecuteQiniuKeyTask(context.Background(), task.Id); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("task_id=%d error=%s", task.Id, sanitizeQiniuTaskError(err)))
			continue
		}
		reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("task_id=%d error=%s", task.Id, sanitizeQiniuTaskError(err)))
			continue
		}
		switch reloadedTask.Status {
		case model.QiniuKeyTaskStatusSuccess:
			result.SuccessCount++
		case model.QiniuKeyTaskStatusSkipped:
			result.SkippedCount++
		}
	}
	return result, nil
}

func StartQiniuKeyTaskRetryTask() {
	qiniuTaskRetryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu key task retry task started: tick=%s", qiniuTaskRetryTickInterval))
			ticker := time.NewTicker(qiniuTaskRetryTickInterval)
			defer ticker.Stop()
			runQiniuKeyTaskRetryOnce()
			for range ticker.C {
				runQiniuKeyTaskRetryOnce()
			}
		})
	})
}

func runQiniuKeyTaskRetryOnce() {
	if !IsQiniuKeyLifecycleEnabled() && !IsQiniuOfficialLedgerEnabled() {
		return
	}
	if !qiniuTaskRetryRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuTaskRetryRunning.Store(false)
	result, err := RetryDueQiniuKeyTasks(qiniuTaskRetryBatchSize)
	if err != nil {
		common.SysLog("qiniu key retry task scan failed: " + err.Error())
		return
	}
	if result.ProcessedCount > 0 || len(result.Errors) > 0 {
		common.SysLog(fmt.Sprintf(
			"qiniu key retry task scanned processed=%d success=%d skipped=%d errors=%d",
			result.ProcessedCount,
			result.SuccessCount,
			result.SkippedCount,
			len(result.Errors),
		))
	}
}

func tryAcquireQiniuTaskLock(key string) (func(), bool) {
	value, _ := qiniuTaskLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		return nil, false
	}
	return func() {
		lock.Unlock()
	}, true
}

func qiniuKeyLifecycleTaskLockKey(keyBody string) string {
	return "key:" + fullQiniuAPIKey(keyBody)
}

func failQiniuTask(taskId int, err error) error {
	safeErr := sanitizeQiniuTaskErrorAsError(err)
	_ = model.MarkQiniuKeyTaskFailed(taskId, qiniuRetryIntervalSeconds(), safeErr)
	return safeErr
}

func failQiniuTaskWithKey(taskId int, tokenId int, keyBody string, err error) error {
	safeErr := sanitizeQiniuTaskErrorAsError(err)
	_ = model.MarkQiniuKeyTaskFailed(taskId, qiniuRetryIntervalSeconds(), safeErr)
	_ = model.DB.Model(&model.QiniuKeySyncTask{}).Where("id = ?", taskId).Updates(map[string]interface{}{
		"token_id":  tokenId,
		"qiniu_key": keyBody,
	}).Error
	return safeErr
}

func logQiniuTaskFailure(taskId int, err error) {
	task, queryErr := model.GetQiniuKeySyncTaskById(taskId)
	if queryErr != nil || task == nil {
		common.SysLog(fmt.Sprintf("qiniu key task failed task_id=%d stage=execute error=%s", taskId, sanitizeQiniuTaskError(err)))
		return
	}
	common.SysLog(fmt.Sprintf(
		"qiniu key task failed task_id=%d task_type=%s user_id=%d token_id=%d stage=execute key=%s error=%s",
		task.Id,
		task.TaskType,
		task.UserId,
		task.TokenId,
		maskQiniuAPIKey(task.QiniuKey),
		sanitizeQiniuTaskError(err),
	))
}

func sanitizeQiniuTaskErrorAsError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(sanitizeQiniuTaskError(err))
}

func sanitizeQiniuTaskError(err error) string {
	if err == nil {
		return ""
	}
	message := SanitizeQiniuChildAccountSecret(err.Error())
	message = strings.TrimSpace(message)
	if len(message) > qiniuTaskErrorMaxLength {
		message = message[:qiniuTaskErrorMaxLength]
	}
	return message
}
