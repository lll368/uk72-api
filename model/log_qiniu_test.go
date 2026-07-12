package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestQiniuLocalObservationLogsAreHiddenFromStandardQueries(t *testing.T) {
	truncateTables(t)

	const userId = 9301
	const tokenId = 9301
	const username = "qiniu_log_filter_user"
	const tokenName = "qiniu-log-token"
	now := common.GetTimestamp()

	require.NoError(t, DB.Create(&User{
		Id:       userId,
		Username: username,
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:          tokenId,
		UserId:      userId,
		Name:        tokenName,
		Key:         "qiniu-log-filter-token",
		Provider:    TokenProviderQiniu,
		Status:      common.TokenStatusEnabled,
		CreatedTime: now,
	}).Error)

	logs := []Log{
		{
			UserId:           userId,
			Username:         username,
			CreatedAt:        now,
			Type:             LogTypeConsume,
			Content:          "ordinary consume",
			TokenName:        tokenName,
			ModelName:        "ordinary-model",
			Quota:            100,
			PromptTokens:     11,
			CompletionTokens: 22,
			TokenId:          tokenId,
			Other:            "{}",
		},
		{
			UserId:           userId,
			Username:         username,
			CreatedAt:        now,
			Type:             LogTypeConsume,
			Content:          "local qiniu observation",
			TokenName:        tokenName,
			ModelName:        "qiniu-local-model",
			Quota:            500,
			PromptTokens:     1000,
			CompletionTokens: 2000,
			TokenId:          tokenId,
			Other: common.MapToJsonStr(map[string]interface{}{
				"billing_source":                 "qiniu_official_ledger",
				"qiniu_official_ledger_pending":  true,
				"qiniu_realtime_billing_skipped": true,
			}),
		},
		{
			UserId:           userId,
			Username:         username,
			CreatedAt:        now,
			Type:             LogTypeConsume,
			Content:          "near miss ordinary log",
			TokenName:        tokenName,
			ModelName:        "near-miss-model",
			Quota:            70,
			PromptTokens:     7,
			CompletionTokens: 8,
			TokenId:          tokenId,
			Other: common.MapToJsonStr(map[string]interface{}{
				"qiniuXofficialXledgerXpending": true,
			}),
		},
		{
			UserId:           userId,
			Username:         username,
			CreatedAt:        now,
			Type:             LogTypeConsume,
			Content:          "official qiniu ledger",
			TokenName:        tokenName,
			ModelName:        "qiniu-official-model",
			Quota:            300,
			PromptTokens:     33,
			CompletionTokens: 44,
			TokenId:          tokenId,
			Other: common.MapToJsonStr(map[string]interface{}{
				"billing_source":            "qiniu_official_ledger",
				"qiniu_official_ledger_log": true,
			}),
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	allLogs, allTotal, err := GetAllLogs(LogTypeConsume, now-1, now+1, "", username, tokenName, 0, 20, 0, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 3, allTotal)
	require.Len(t, allLogs, 3)
	require.ElementsMatch(t, []string{"ordinary consume", "near miss ordinary log", "official qiniu ledger"}, logContents(allLogs))

	userLogs, userTotal, err := GetUserLogs(userId, LogTypeConsume, now-1, now+1, "", tokenName, 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 3, userTotal)
	require.Len(t, userLogs, 3)
	require.ElementsMatch(t, []string{"ordinary consume", "near miss ordinary log", "official qiniu ledger"}, logContents(userLogs))

	tokenLogs, err := GetLogByTokenId(tokenId)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"ordinary consume", "near miss ordinary log", "official qiniu ledger"}, logContents(tokenLogs))

	stat, err := SumUsedQuota(LogTypeConsume, now-1, now+1, "", username, tokenName, 0, "")
	require.NoError(t, err)
	require.Equal(t, 470, stat.Quota)
	require.Equal(t, 3, stat.Rpm)
	require.Equal(t, 125, stat.Tpm)
}

func TestUserLogsHideManageAuditLogs(t *testing.T) {
	truncateTables(t)

	const userId = 9401
	const username = "manage_log_hidden_user"
	now := common.GetTimestamp()

	require.NoError(t, DB.Create(&User{
		Id:       userId,
		Username: username,
		Status:   common.UserStatusEnabled,
	}).Error)

	logs := []Log{
		{
			UserId:    userId,
			Username:  username,
			CreatedAt: now,
			Type:      LogTypeTopup,
			Content:   "user topup log",
		},
		{
			UserId:    userId,
			Username:  username,
			CreatedAt: now,
			Type:      LogTypeSystem,
			Content:   "user system log",
		},
		{
			UserId:    userId,
			Username:  username,
			CreatedAt: now,
			Type:      LogTypeManage,
			Content:   "算力伙伴 调整下级充值折扣",
			Other: common.MapToJsonStr(map[string]interface{}{
				"admin_info": map[string]interface{}{
					"parent_user_id": 1001,
					"child_user_id":  userId,
				},
			}),
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	userLogs, userTotal, err := GetUserLogs(userId, LogTypeUnknown, now-1, now+1, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, userTotal)
	require.ElementsMatch(t, []string{"user topup log", "user system log"}, logContents(userLogs))

	userManageLogs, userManageTotal, err := GetUserLogs(userId, LogTypeManage, now-1, now+1, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 0, userManageTotal)
	require.Empty(t, userManageLogs)

	adminLogs, adminTotal, err := GetAllLogs(LogTypeManage, now-1, now+1, "", username, "", 0, 20, 0, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, adminTotal)
	require.ElementsMatch(t, []string{"算力伙伴 调整下级充值折扣"}, logContents(adminLogs))
}

func logContents(logs []*Log) []string {
	contents := make([]string, 0, len(logs))
	for _, log := range logs {
		contents = append(contents, log.Content)
	}
	return contents
}
