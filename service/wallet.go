package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWalletAmountInvalid       = errors.New("钱包金额必须大于 0")
	ErrWalletBalanceInsufficient = errors.New("消费余额不足")
	ErrCommissionInsufficient    = errors.New("可提现佣金不足")
)

const walletAmountEpsilon = 0.000001
const walletTransactionMaxAttempts = 3
const walletTransactionRetryBaseDelay = 25 * time.Millisecond

type walletTransactionExecutor func(operation func(tx *gorm.DB) error) error

func amountToQuota(amount float64) int {
	if amount <= 0 {
		return 0
	}
	return int(decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
}

func quotaToWalletAmount(quota int) float64 {
	if quota <= 0 {
		return 0
	}
	return decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(common.QuotaPerUnit)).InexactFloat64()
}

func walletLockQuery(tx *gorm.DB) *gorm.DB {
	if common.UsingSQLite {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

func walletIdempotencyKey(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func runWalletTransactionWithRetry(operation func(tx *gorm.DB) error) error {
	return runWalletTransactionWithRetryUsingExecutor(operation, func(operation func(tx *gorm.DB) error) error {
		return model.DB.Transaction(operation)
	})
}

func runWalletTransactionWithRetryUsingExecutor(operation func(tx *gorm.DB) error, execute walletTransactionExecutor) error {
	if operation == nil {
		return errors.New("钱包事务不能为空")
	}
	if execute == nil {
		return errors.New("钱包事务执行器不能为空")
	}
	var err error
	for attempt := 1; attempt <= walletTransactionMaxAttempts; attempt++ {
		err = execute(operation)
		if err == nil {
			return nil
		}
		if !isWalletRetryableTransactionError(err) || attempt == walletTransactionMaxAttempts {
			return err
		}
		// 钱包页会并发触发账户懒初始化，MySQL 死锁/锁等待属于可恢复事务错误，短暂退避后重试。
		time.Sleep(time.Duration(attempt) * walletTransactionRetryBaseDelay)
	}
	return err
}

func isWalletRetryableTransactionError(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1213 || mysqlErr.Number == 1205
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "error 1213") ||
		strings.Contains(message, "deadlock found") ||
		strings.Contains(message, "try restarting transaction") ||
		strings.Contains(message, "lock wait timeout") ||
		strings.Contains(message, "sqlstate 40001") ||
		strings.Contains(message, "sqlstate 40p01") ||
		strings.Contains(message, "deadlock detected")
}

func walletFlowExistsTx(tx *gorm.DB, idempotencyKey string) (bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return false, nil
	}
	var count int64
	if err := tx.Model(&model.WalletFlow{}).Where("idempotency_key = ?", idempotencyKey).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func getOrCreateWalletAccountTx(tx *gorm.DB, userId int, lock bool) (*model.WalletAccount, error) {
	if tx == nil {
		tx = model.DB
	}
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	var account model.WalletAccount
	query := tx.Where("user_id = ?", userId)
	if lock {
		query = walletLockQuery(query)
	}
	if err := query.First(&account).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		account = model.WalletAccount{UserId: userId}
		if err := tx.Create(&account).Error; err != nil {
			if retryErr := tx.Where("user_id = ?", userId).First(&account).Error; retryErr != nil {
				return nil, err
			}
		}
		if lock && !common.UsingSQLite {
			if err := walletLockQuery(tx).Where("user_id = ?", userId).First(&account).Error; err != nil {
				return nil, err
			}
		}
	}
	return &account, nil
}

// GetOrCreateWalletAccount 查询或创建用户钱包账户，并对旧用户 quota 做一次幂等余额回填。
func GetOrCreateWalletAccount(userId int) (*model.WalletAccount, error) {
	var account *model.WalletAccount
	err := runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		var err error
		account, err = ensureLegacyWalletBalanceTx(tx, userId)
		return err
	})
	return account, err
}

func ensureLegacyWalletBalanceTx(tx *gorm.DB, userId int) (*model.WalletAccount, error) {
	account, err := getOrCreateWalletAccountTx(tx, userId, true)
	if err != nil {
		return nil, err
	}
	if account.BalanceAmount > walletAmountEpsilon {
		return account, nil
	}
	legacyKey := fmt.Sprintf("wallet:legacy-balance-init:%d", userId)
	exists, err := walletFlowExistsTx(tx, legacyKey)
	if err != nil || exists {
		return account, err
	}
	var user model.User
	if err := walletLockQuery(tx).Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	if user.Quota <= 0 {
		return account, nil
	}
	amount := quotaToWalletAmount(user.Quota)
	account.BalanceAmount = amount
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}
	if err := createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                userId,
		BizNo:                 fmt.Sprintf("legacy-balance-init-%d", userId),
		IdempotencyKey:        walletIdempotencyKey(legacyKey),
		FlowType:              model.WalletFlowTypeLegacyBalanceInit,
		WalletType:            model.WalletTypeBalance,
		Direction:             model.WalletFlowDirectionIn,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                "历史 quota 初始化消费余额",
	}); err != nil {
		return nil, err
	}
	return account, nil
}

func createWalletFlowTx(tx *gorm.DB, flow *model.WalletFlow) error {
	if tx == nil {
		tx = model.DB
	}
	if flow == nil {
		return errors.New("钱包流水不能为空")
	}
	if flow.IdempotencyKey != nil && strings.TrimSpace(*flow.IdempotencyKey) != "" {
		exists, err := walletFlowExistsTx(tx, *flow.IdempotencyKey)
		if err != nil || exists {
			return err
		}
	}
	return tx.Create(flow).Error
}

func creditBalanceByQuotaTx(tx *gorm.DB, userId int, quota int, bizNo string, flowType string, remark string, idempotencyKey string) error {
	if quota <= 0 {
		return nil
	}
	exists, err := walletFlowExistsTx(tx, idempotencyKey)
	if err != nil || exists {
		return err
	}
	amount := quotaToWalletAmount(quota)
	account, err := ensureLegacyWalletBalanceTx(tx, userId)
	if err != nil {
		return err
	}
	account.BalanceAmount += amount
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", quota)).Error; err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                userId,
		BizNo:                 bizNo,
		IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
		FlowType:              flowType,
		WalletType:            model.WalletTypeBalance,
		Direction:             model.WalletFlowDirectionIn,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                remark,
	})
}

func debitBalanceByQuotaTx(tx *gorm.DB, userId int, quota int, bizNo string, flowType string, remark string, idempotencyKey string) error {
	if quota <= 0 {
		return nil
	}
	exists, err := walletFlowExistsTx(tx, idempotencyKey)
	if err != nil || exists {
		return err
	}
	account, err := ensureLegacyWalletBalanceTx(tx, userId)
	if err != nil {
		return err
	}
	amount := quotaToWalletAmount(quota)
	if account.BalanceAmount+walletAmountEpsilon < amount {
		return ErrWalletBalanceInsufficient
	}
	account.BalanceAmount = math.Max(0, account.BalanceAmount-amount)
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota - ?", quota)).Error; err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                userId,
		BizNo:                 bizNo,
		IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
		FlowType:              flowType,
		WalletType:            model.WalletTypeBalance,
		Direction:             model.WalletFlowDirectionOut,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                remark,
	})
}

func getQiniuWalletAccountAndUserTx(tx *gorm.DB, userId int) (*model.WalletAccount, *model.User, error) {
	account, err := getOrCreateWalletAccountTx(tx, userId, true)
	if err != nil {
		return nil, nil, err
	}
	var user model.User
	if err := walletLockQuery(tx).Select("id", "quota").Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, nil, err
	}
	if account.BalanceAmount <= walletAmountEpsilon && user.Quota > 0 {
		// 七牛实时请求的内部资金源只做静默账务保护，不创建历史余额初始化流水。
		account.BalanceAmount = quotaToWalletAmount(user.Quota)
		if err := tx.Save(account).Error; err != nil {
			return nil, nil, err
		}
	}
	return account, &user, nil
}

func debitBalanceByQuotaSilentlyTx(tx *gorm.DB, userId int, quota int) error {
	if quota <= 0 {
		return nil
	}
	account, user, err := getQiniuWalletAccountAndUserTx(tx, userId)
	if err != nil {
		return err
	}
	amount := quotaToWalletAmount(quota)
	if user.Quota < quota || account.BalanceAmount+walletAmountEpsilon < amount {
		return ErrWalletBalanceInsufficient
	}
	account.BalanceAmount = math.Max(0, account.BalanceAmount-amount)
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	return tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", user.Quota-quota).Error
}

func debitBalanceByQuotaSilentlyAllowDebtTx(tx *gorm.DB, userId int, quota int) error {
	if quota <= 0 {
		return nil
	}
	account, _, err := getQiniuWalletAccountAndUserTx(tx, userId)
	if err != nil {
		return err
	}
	amount := quotaToWalletAmount(quota)
	// 七牛上游成功后实际消费已经发生，后结算差额必须落账；余额不足时形成 debt。
	account.BalanceAmount -= amount
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	return tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota - ?", quota)).Error
}

func creditBalanceByQuotaSilentlyTx(tx *gorm.DB, userId int, quota int) error {
	if quota <= 0 {
		return nil
	}
	account, user, err := getQiniuWalletAccountAndUserTx(tx, userId)
	if err != nil {
		return err
	}
	account.BalanceAmount += quotaToWalletAmount(quota)
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	return tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", user.Quota+quota).Error
}

func creditCommissionTx(tx *gorm.DB, userId int, bizNo string, amount float64, flowType string, remark string, idempotencyKey string) error {
	if amount <= 0 {
		return nil
	}
	exists, err := walletFlowExistsTx(tx, idempotencyKey)
	if err != nil || exists {
		return err
	}
	account, err := getOrCreateWalletAccountTx(tx, userId, true)
	if err != nil {
		return err
	}
	account.CommissionAmount += amount
	account.TotalCommissionAmount += amount
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                userId,
		BizNo:                 bizNo,
		IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
		FlowType:              flowType,
		WalletType:            model.WalletTypeCommission,
		Direction:             model.WalletFlowDirectionIn,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                remark,
	})
}

// TransferCommissionToBalance 将可提现佣金转为消费余额，转入后只能用于消费。
func TransferCommissionToBalance(userId int, amount float64) error {
	if userId <= 0 {
		return errors.New("用户不存在")
	}
	if amount <= 0 {
		return ErrWalletAmountInvalid
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		account, err := ensureLegacyWalletBalanceTx(tx, userId)
		if err != nil {
			return err
		}
		if account.CommissionAmount+walletAmountEpsilon < amount {
			return ErrCommissionInsufficient
		}
		account.CommissionAmount -= amount
		account.BalanceAmount += amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		quota := amountToQuota(amount)
		if quota <= 0 {
			return ErrWalletAmountInvalid
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", quota)).Error; err != nil {
			return err
		}
		flow := &model.WalletFlow{
			UserId:                userId,
			BizNo:                 fmt.Sprintf("commission-transfer-%d", userId),
			IdempotencyKey:        walletIdempotencyKey(fmt.Sprintf("wallet:commission-transfer:%d:%s", userId, common.GetTimeString())),
			FlowType:              model.WalletFlowTypeCommissionToBalance,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionOut,
			Amount:                amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                "佣金转消费余额",
		}
		if err := createWalletFlowTx(tx, flow); err != nil {
			return err
		}
		if err := createQiniuQuotaGrantForCommissionTransferTx(tx, userId, flow.Id, amount); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func consumeWalletQuota(userId int, quota int, bizNo string, remark string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		return debitBalanceByQuotaTx(tx, userId, quota, bizNo, model.WalletFlowTypeBalanceConsume, remark, "")
	})
}

func refundWalletQuota(userId int, quota int, bizNo string, remark string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		return creditBalanceByQuotaTx(tx, userId, quota, bizNo, model.WalletFlowTypeBalanceRefund, remark, "")
	})
}

func consumeQiniuWalletQuotaSilently(userId int, quota int) error {
	return runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		return debitBalanceByQuotaSilentlyTx(tx, userId, quota)
	})
}

func consumeQiniuWalletQuotaSilentlyAllowDebt(userId int, quota int) error {
	return runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		return debitBalanceByQuotaSilentlyAllowDebtTx(tx, userId, quota)
	})
}

func refundQiniuWalletQuotaSilently(userId int, quota int) error {
	return runWalletTransactionWithRetry(func(tx *gorm.DB) error {
		return creditBalanceByQuotaSilentlyTx(tx, userId, quota)
	})
}
