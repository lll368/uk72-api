package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type QiniuBillingBucketAdminOperationResult struct {
	Action        string `json:"action"`
	BucketId      int    `json:"bucket_id"`
	RawRecordId   int    `json:"raw_record_id,omitempty"`
	ApplicationId int    `json:"application_id,omitempty"`
	TokenId       int    `json:"token_id,omitempty"`
	BillingDate   string `json:"billing_date,omitempty"`
	Status        string `json:"status,omitempty"`
	Applied       bool   `json:"applied"`
	Skipped       bool   `json:"skipped"`
	Message       string `json:"message"`
}

func AdminRecalculateQiniuBillingBucket(ctx context.Context, bucketId int, adminUserId int, reason string) (*QiniuBillingBucketAdminOperationResult, error) {
	if bucketId <= 0 {
		return nil, errors.New("账单 bucket ID 无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var current model.QiniuBillingBucket
	if err := model.DB.Select("id", "user_id", "token_id", "billing_date").Where("id = ?", bucketId).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("账单 bucket 不存在")
		}
		return nil, err
	}
	bucket, err := RecalculateQiniuBillingBucket(current.TokenId, current.BillingDate)
	if err != nil {
		return nil, err
	}
	model.RecordLogWithAdminInfo(current.UserId, model.LogTypeManage, "管理员重算 cost-detail bucket", map[string]interface{}{
		"admin_user_id": adminUserId,
		"bucket_id":     bucketId,
		"token_id":      current.TokenId,
		"billing_date":  current.BillingDate,
		"reason":        strings.TrimSpace(reason),
	})
	return &QiniuBillingBucketAdminOperationResult{
		Action:      "recalculate",
		BucketId:    bucket.Id,
		TokenId:     bucket.TokenId,
		BillingDate: bucket.BillingDate,
		Status:      bucket.Status,
		Message:     qiniuBucketRecalculateMessage(bucket),
	}, nil
}

func AdminRetryQiniuBillingBucketApplication(ctx context.Context, applicationId int, adminUserId int, reason string) (*QiniuBillingBucketAdminOperationResult, error) {
	if applicationId <= 0 {
		return nil, errors.New("账单 bucket application ID 无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var application model.QiniuBillingBucketApplication
	if err := model.DB.Select("id", "bucket_id").Where("id = ?", applicationId).First(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("账单 bucket application 不存在")
		}
		return nil, err
	}
	applied, skipped, err := applyQiniuBillingBucket(ctx, application.BucketId, qiniuBillingBucketApplyOptions{
		ExpectedApplicationId: application.Id,
		OperationSource:       model.QiniuBillingOperationSourceAdmin,
	})
	if err != nil {
		return nil, err
	}
	var bucket model.QiniuBillingBucket
	if err := model.DB.Select("id", "user_id", "token_id", "billing_date").Where("id = ?", application.BucketId).First(&bucket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("账单 bucket 不存在")
		}
		return nil, err
	}
	model.RecordLogWithAdminInfo(bucket.UserId, model.LogTypeManage, "管理员重试 cost-detail bucket application", map[string]interface{}{
		"admin_user_id":  adminUserId,
		"application_id": applicationId,
		"bucket_id":      bucket.Id,
		"token_id":       bucket.TokenId,
		"billing_date":   bucket.BillingDate,
		"reason":         strings.TrimSpace(reason),
	})
	message := "已重读当前 bucket 并重试应用 pending delta"
	if skipped {
		message = "当前 bucket 无需重复应用，已跳过重复扣费/退款"
	}
	return &QiniuBillingBucketAdminOperationResult{
		Action:        "retry_application",
		BucketId:      bucket.Id,
		ApplicationId: application.Id,
		TokenId:       bucket.TokenId,
		BillingDate:   bucket.BillingDate,
		Applied:       applied,
		Skipped:       skipped,
		Message:       message,
	}, nil
}

func AdminSkipQiniuBillingBucket(ctx context.Context, bucketId int, adminUserId int, reason string) (*QiniuBillingBucketAdminOperationResult, error) {
	if bucketId <= 0 {
		return nil, errors.New("账单 bucket ID 无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("跳过账单 bucket 必须填写原因")
	}
	var bucket model.QiniuBillingBucket
	if err := model.DB.First(&bucket, "id = ?", bucketId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("账单 bucket 不存在")
		}
		return nil, err
	}
	if bucket.Status == model.QiniuBillingBucketStatusApplied || bucket.Status == model.QiniuBillingBucketStatusReconciled {
		return nil, errors.New("已应用或已对账的账单 bucket 不能跳过")
	}
	if err := model.DB.Model(&model.QiniuBillingBucket{}).Where("id = ?", bucket.Id).Updates(map[string]interface{}{
		"status":       model.QiniuBillingBucketStatusSkipped,
		"last_error":   "admin_skip: " + reason,
		"updated_time": common.GetTimestamp(),
	}).Error; err != nil {
		return nil, err
	}
	model.RecordLogWithAdminInfo(bucket.UserId, model.LogTypeManage, "管理员跳过 cost-detail bucket", map[string]interface{}{
		"admin_user_id":       adminUserId,
		"bucket_id":           bucket.Id,
		"token_id":            bucket.TokenId,
		"billing_date":        bucket.BillingDate,
		"pending_delta_quota": bucket.PendingDeltaQuota,
		"reason":              reason,
	})
	return &QiniuBillingBucketAdminOperationResult{
		Action:      "skip",
		BucketId:    bucket.Id,
		TokenId:     bucket.TokenId,
		BillingDate: bucket.BillingDate,
		Status:      model.QiniuBillingBucketStatusSkipped,
		Skipped:     true,
		Message:     fmt.Sprintf("已跳过 bucket，pending_delta_quota=%d 不会自动扣费或退款；后续重算可重新进入待处理状态", bucket.PendingDeltaQuota),
	}, nil
}

func AdminResolveQiniuCostDetailRecord(ctx context.Context, input QiniuManualOwnershipInput) (*QiniuBillingBucketAdminOperationResult, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	bucket, err := ManuallyResolveQiniuCostDetailRawRecordOwnership(input)
	if err != nil {
		return nil, err
	}
	return &QiniuBillingBucketAdminOperationResult{
		Action:      "manual_resolve",
		BucketId:    bucket.Id,
		RawRecordId: input.RawRecordId,
		TokenId:     bucket.TokenId,
		BillingDate: bucket.BillingDate,
		Status:      bucket.Status,
		Message:     qiniuBucketRecalculateMessage(bucket),
	}, nil
}

func AdminResolveQiniuBillingBucket(ctx context.Context, input QiniuManualBucketOwnershipInput) (*QiniuBillingBucketAdminOperationResult, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	bucket, err := ManuallyResolveQiniuBillingBucketOwnership(input)
	if err != nil {
		return nil, err
	}
	return &QiniuBillingBucketAdminOperationResult{
		Action:      "manual_resolve_bucket",
		BucketId:    bucket.Id,
		TokenId:     bucket.TokenId,
		BillingDate: bucket.BillingDate,
		Status:      bucket.Status,
		Message:     qiniuBucketRecalculateMessage(bucket),
	}, nil
}

func qiniuBucketRecalculateMessage(bucket *model.QiniuBillingBucket) string {
	if bucket == nil {
		return "bucket 重算完成"
	}
	return fmt.Sprintf("bucket 已重算，official_quota=%d local_realtime_quota=%d applied_delta_quota=%d pending_delta_quota=%d status=%s",
		bucket.OfficialQuota,
		bucket.LocalRealtimeQuota,
		bucket.AppliedDeltaQuota,
		bucket.PendingDeltaQuota,
		bucket.Status,
	)
}
