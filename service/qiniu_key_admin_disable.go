package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var (
	getAdminDisableQiniuTokenById          = getAdminDisableQiniuTokenByIdFromDB
	invalidateAdminDisabledQiniuTokenCache = model.InvalidateUserTokensCache
)

// AdminDisableQiniuKey 通过七牛远端接口禁用托管 Key，远端成功后再更新本地状态。
func AdminDisableQiniuKey(ctx context.Context, tokenId int, adminUserId int, reason string) (*model.Token, error) {
	if tokenId <= 0 {
		return nil, errors.New("Key ID 无效")
	}
	token, err := getAdminDisableQiniuTokenById(tokenId)
	if err != nil {
		return nil, err
	}
	if !IsQiniuManagedToken(token) {
		return nil, errors.New("只能禁用七牛托管 Key")
	}
	if token.Status != common.TokenStatusEnabled {
		return nil, errors.New("只有启用状态的七牛托管 Key 可以禁用")
	}
	if !isQiniuAPIKeyBody(token.Key) {
		return nil, errors.New("七牛托管 Key 格式无效")
	}

	client, err := newQiniuKeyClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return nil, err
	}
	if err := client.SetAPIKeyEnabled(ctx, token.Key, false); err != nil {
		common.SysLog(fmt.Sprintf("admin qiniu key disable failed user_id=%d token_id=%d key=%s err=%s", token.UserId, token.Id, maskQiniuAPIKey(token.Key), err.Error()))
		return nil, fmt.Errorf("七牛远端禁用失败: %w", err)
	}

	// 远端已经确认禁用后才更新本地状态，避免远端失败时误伤现有用户请求。
	token.Status = common.TokenStatusDisabled
	if err := token.Update(); err != nil {
		common.SysLog(fmt.Sprintf("admin qiniu key local disable failed after remote success user_id=%d token_id=%d key=%s key_fingerprint=%s admin_user_id=%d err=%s", token.UserId, token.Id, maskQiniuAPIKey(token.Key), model.QiniuTokenKeyFingerprint(token.Key), adminUserId, err.Error()))
		return nil, fmt.Errorf("七牛远端已禁用，但本地状态更新失败: %w", err)
	}

	trimmedReason := strings.TrimSpace(reason)
	model.RecordLogWithAdminInfo(token.UserId, model.LogTypeManage, "管理员禁用七牛托管 Key", map[string]interface{}{
		"admin_user_id":   adminUserId,
		"token_id":        token.Id,
		"user_id":         token.UserId,
		"key":             model.MaskTokenKey(token.Key),
		"key_fingerprint": model.QiniuTokenKeyFingerprint(token.Key),
		"reason":          trimmedReason,
	})
	if err := invalidateAdminDisabledQiniuTokenCache(token.UserId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate qiniu token cache after admin disable user_id=%d token_id=%d key=%s key_fingerprint=%s admin_user_id=%d err=%s", token.UserId, token.Id, maskQiniuAPIKey(token.Key), model.QiniuTokenKeyFingerprint(token.Key), adminUserId, err.Error()))
	}
	common.SysLog(fmt.Sprintf("admin qiniu key disabled user_id=%d token_id=%d key=%s admin_user_id=%d", token.UserId, token.Id, maskQiniuAPIKey(token.Key), adminUserId))
	return token, nil
}

func getAdminDisableQiniuTokenByIdFromDB(tokenId int) (*model.Token, error) {
	if tokenId <= 0 {
		return nil, errors.New("id 为空！")
	}
	token := model.Token{Id: tokenId}
	// 管理员禁用会在远端成功后清理 token cache；这里不能使用 model.GetTokenById，
	// 否则禁用前的 enabled 状态可能被异步写回 Redis，覆盖后续缓存清理结果。
	if err := model.DB.First(&token, "id = ?", tokenId).Error; err != nil {
		return nil, err
	}
	return &token, nil
}
