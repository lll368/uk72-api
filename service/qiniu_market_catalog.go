package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	QiniuMarketCatalogStatusDisabled      = "disabled"
	QiniuMarketCatalogStatusFresh         = "fresh"
	QiniuMarketCatalogStatusStale         = "stale"
	QiniuMarketCatalogStatusFallbackLocal = "fallback_local"
	QiniuMarketPriceSource                = "qiniu_market"
	QiniuMarketPriceSourceLabel           = "官方市场价"
	qiniuMarketCatalogRefreshTickInterval = 30 * time.Minute
)

var qiniuMarketSecretPattern = regexp.MustCompile(`(?i)\b(access_key|secret|secret_key|ak|sk)=\S+`)

var (
	qiniuMarketCatalogRefreshOnce    sync.Once
	qiniuMarketCatalogRefreshRunning atomic.Bool
)

type qiniuMarketCatalogFetcher func(context.Context) ([]dto.QiniuMarketModel, error)

type qiniuMarketCatalogSnapshotMode int

const (
	qiniuMarketCatalogSnapshotFreshOrFetch qiniuMarketCatalogSnapshotMode = iota
	qiniuMarketCatalogSnapshotForceRefresh
	qiniuMarketCatalogSnapshotCurrentOrFetch
	qiniuMarketCatalogSnapshotCurrentOnly
)

type qiniuMarketCatalogCacheState struct {
	mu              sync.Mutex
	cacheKey        string
	models          []dto.QiniuMarketModel
	lastSuccessTime time.Time
	expiresAt       time.Time
	lastError       string
	fetcher         qiniuMarketCatalogFetcher
}

var qiniuMarketCatalogCache = &qiniuMarketCatalogCacheState{
	fetcher: fetchQiniuMarketCatalogModels,
}

// QiniuMarketCatalogSnapshot 表示一次模型市场目录读取结果。
type QiniuMarketCatalogSnapshot struct {
	Status          string
	Models          []dto.QiniuMarketModel
	LastSuccessTime time.Time
	LastError       string
	Stale           bool
	FromCache       bool
	FallbackLocal   bool
	Strict          bool `json:"-"`
}

type QiniuMarketCatalogState struct {
	Status          string `json:"status"`
	LastSuccessTime int64  `json:"last_success_time,omitempty"`
	Stale           bool   `json:"stale,omitempty"`
	FromCache       bool   `json:"from_cache,omitempty"`
	FallbackLocal   bool   `json:"fallback_local,omitempty"`
}

func StartQiniuMarketCatalogRefreshTask() {
	qiniuMarketCatalogRefreshOnce.Do(func() {
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu market catalog refresh task started: tick=%s", qiniuMarketCatalogRefreshTickInterval))
			ticker := time.NewTicker(qiniuMarketCatalogRefreshTickInterval)
			defer ticker.Stop()
			runQiniuMarketCatalogRefreshOnce()
			for range ticker.C {
				runQiniuMarketCatalogRefreshOnce()
			}
		})
	})
}

func runQiniuMarketCatalogRefreshOnce() {
	setting := operation_setting.GetQiniuKeySetting()
	if setting == nil || !setting.MarketCatalogEnabled {
		return
	}
	if !qiniuMarketCatalogRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuMarketCatalogRefreshRunning.Store(false)

	timeout := setting.RequestTimeout
	if timeout <= 0 {
		timeout = operation_setting.QiniuKeyDefaultRequestTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	snapshot := RefreshQiniuMarketCatalogOnce(ctx)
	if snapshot.Status == QiniuMarketCatalogStatusFresh {
		return
	}
	common.SysLog(fmt.Sprintf(
		"qiniu market catalog refresh unavailable status=%s stale=%t fallback_local=%t last_error=%s",
		snapshot.Status,
		snapshot.Stale,
		snapshot.FallbackLocal,
		snapshot.LastError,
	))
}

// RefreshQiniuMarketCatalogOnce 主动刷新当前进程内七牛模型市场快照。
// 每个 API 进程都需要本地快照，不能只依赖 master 节点或用户访问模型广场预热。
func RefreshQiniuMarketCatalogOnce(ctx context.Context) QiniuMarketCatalogSnapshot {
	return getQiniuMarketCatalogSnapshot(ctx, qiniuMarketCatalogSnapshotForceRefresh)
}

// GetQiniuMarketCatalogSnapshot 获取七牛模型市场快照，接口失败时优先使用最后成功快照。
func GetQiniuMarketCatalogSnapshot(ctx context.Context) QiniuMarketCatalogSnapshot {
	return getQiniuMarketCatalogSnapshot(ctx, qiniuMarketCatalogSnapshotFreshOrFetch)
}

func getQiniuMarketCatalogSnapshot(ctx context.Context, mode qiniuMarketCatalogSnapshotMode) QiniuMarketCatalogSnapshot {
	if ctx == nil {
		ctx = context.Background()
	}
	setting := operation_setting.GetQiniuKeySetting()
	if setting == nil || !setting.MarketCatalogEnabled {
		return QiniuMarketCatalogSnapshot{Status: QiniuMarketCatalogStatusDisabled}
	}

	now := time.Now()
	strict := !setting.MarketCatalogFallbackEnabled
	cacheKey := qiniuMarketCatalogCacheKey(setting)
	qiniuMarketCatalogCache.mu.Lock()
	if mode != qiniuMarketCatalogSnapshotForceRefresh && qiniuMarketCatalogCache.cacheKey == cacheKey && len(qiniuMarketCatalogCache.models) > 0 {
		status := QiniuMarketCatalogStatusFresh
		stale := false
		if !now.Before(qiniuMarketCatalogCache.expiresAt) {
			status = QiniuMarketCatalogStatusStale
			stale = true
		}
		if mode == qiniuMarketCatalogSnapshotFreshOrFetch && stale {
			fetcher := qiniuMarketCatalogCache.fetcher
			qiniuMarketCatalogCache.mu.Unlock()
			return fetchQiniuMarketCatalogSnapshot(ctx, setting, cacheKey, strict, now, fetcher)
		}
		snapshot := QiniuMarketCatalogSnapshot{
			Status:          status,
			Models:          cloneQiniuMarketModels(qiniuMarketCatalogCache.models),
			LastSuccessTime: qiniuMarketCatalogCache.lastSuccessTime,
			LastError:       qiniuMarketCatalogCache.lastError,
			Stale:           stale,
			FromCache:       true,
			Strict:          strict,
		}
		qiniuMarketCatalogCache.mu.Unlock()
		return snapshot
	}
	if mode == qiniuMarketCatalogSnapshotCurrentOnly {
		status := QiniuMarketCatalogStatusFallbackLocal
		if strict {
			status = QiniuMarketCatalogStatusStale
		}
		snapshot := QiniuMarketCatalogSnapshot{
			Status:        status,
			LastError:     qiniuMarketCatalogCache.lastError,
			Stale:         status == QiniuMarketCatalogStatusStale,
			FallbackLocal: status == QiniuMarketCatalogStatusFallbackLocal,
			Strict:        strict,
		}
		qiniuMarketCatalogCache.mu.Unlock()
		return snapshot
	}
	fetcher := qiniuMarketCatalogCache.fetcher
	qiniuMarketCatalogCache.mu.Unlock()

	return fetchQiniuMarketCatalogSnapshot(ctx, setting, cacheKey, strict, now, fetcher)
}

func fetchQiniuMarketCatalogSnapshot(ctx context.Context, setting *operation_setting.QiniuKeySetting, cacheKey string, strict bool, now time.Time, fetcher qiniuMarketCatalogFetcher) QiniuMarketCatalogSnapshot {
	if fetcher == nil {
		fetcher = fetchQiniuMarketCatalogModels
	}
	models, err := fetcher(ctx)
	if err == nil {
		ttl := time.Duration(setting.MarketCatalogTTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = time.Duration(operation_setting.QiniuMarketCatalogDefaultTTLSeconds) * time.Second
		}
		qiniuMarketCatalogCache.mu.Lock()
		qiniuMarketCatalogCache.cacheKey = cacheKey
		qiniuMarketCatalogCache.models = cloneQiniuMarketModels(models)
		qiniuMarketCatalogCache.lastSuccessTime = now
		qiniuMarketCatalogCache.expiresAt = now.Add(ttl)
		qiniuMarketCatalogCache.lastError = ""
		qiniuMarketCatalogCache.mu.Unlock()
		return QiniuMarketCatalogSnapshot{
			Status:          QiniuMarketCatalogStatusFresh,
			Models:          cloneQiniuMarketModels(models),
			LastSuccessTime: now,
			Strict:          strict,
		}
	}

	lastError := sanitizeQiniuMarketError(err)
	qiniuMarketCatalogCache.mu.Lock()
	qiniuMarketCatalogCache.lastError = lastError
	if qiniuMarketCatalogCache.cacheKey == cacheKey && len(qiniuMarketCatalogCache.models) > 0 {
		snapshot := QiniuMarketCatalogSnapshot{
			Status:          QiniuMarketCatalogStatusStale,
			Models:          cloneQiniuMarketModels(qiniuMarketCatalogCache.models),
			LastSuccessTime: qiniuMarketCatalogCache.lastSuccessTime,
			LastError:       lastError,
			Stale:           true,
			Strict:          strict,
		}
		qiniuMarketCatalogCache.mu.Unlock()
		return snapshot
	}
	qiniuMarketCatalogCache.mu.Unlock()

	status := QiniuMarketCatalogStatusFallbackLocal
	if !setting.MarketCatalogFallbackEnabled {
		status = QiniuMarketCatalogStatusStale
	}
	return QiniuMarketCatalogSnapshot{
		Status:        status,
		LastError:     lastError,
		Stale:         status == QiniuMarketCatalogStatusStale,
		FallbackLocal: status == QiniuMarketCatalogStatusFallbackLocal,
		Strict:        strict,
	}
}

// GetCurrentQiniuMarketCatalogSnapshot 获取当前进程可用的七牛模型市场快照。
// 内存已有快照时不刷新；内存没有数据时兜底拉取一次，避免后台任务未启动导致首个请求失败。
func GetCurrentQiniuMarketCatalogSnapshot(ctx context.Context) QiniuMarketCatalogSnapshot {
	return getQiniuMarketCatalogSnapshot(ctx, qiniuMarketCatalogSnapshotCurrentOrFetch)
}

// GetCachedQiniuMarketCatalogSnapshot 只读取当前进程内的七牛模型市场快照，不触发远程刷新。
// 该方法用于错误日志等诊断路径，避免记录日志时再次调用七牛接口并改变失败现场。
func GetCachedQiniuMarketCatalogSnapshot() QiniuMarketCatalogSnapshot {
	return getQiniuMarketCatalogSnapshot(context.Background(), qiniuMarketCatalogSnapshotCurrentOnly)
}

func fetchQiniuMarketCatalogModels(ctx context.Context) ([]dto.QiniuMarketModel, error) {
	client, err := newQiniuKeyClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return nil, err
	}
	return client.QueryMarketModels(ctx)
}

// FetchQiniuOverseasMarketModels 直接读取七牛海外模型市场列表，不依赖后台模型市场同步开关。
func FetchQiniuOverseasMarketModels(ctx context.Context) ([]dto.QiniuMarketModel, error) {
	setting := operation_setting.GetQiniuKeySetting()
	requestTimeout := operation_setting.QiniuKeyDefaultRequestTimeout
	marketBaseURL := operation_setting.QiniuMarketDefaultBaseURL
	if setting != nil {
		if setting.RequestTimeout > 0 {
			requestTimeout = setting.RequestTimeout
		}
		if strings.TrimSpace(setting.MarketCatalogBaseURL) != "" {
			marketBaseURL = setting.MarketCatalogBaseURL
		}
	}
	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		MarketCatalogEnabled:  true,
		MarketCatalogBaseURL:  marketBaseURL,
		MarketCatalogOverseas: true,
		RequestTimeout:        requestTimeout,
	})
	if err != nil {
		return nil, err
	}
	return client.QueryMarketModels(ctx)
}

func qiniuMarketCatalogCacheKey(setting *operation_setting.QiniuKeySetting) string {
	if setting == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(setting.MarketCatalogBaseURL), "/") + "|overseas=" + strconv.FormatBool(setting.MarketCatalogOverseas)
}

// ApplyQiniuMarketCatalogToPricing 将七牛 market 信息合并到本地已可路由模型上。
// 严格模式下只保留七牛 market 命中的本地可路由模型，避免模型广场回退展示旧本地数据。
func ApplyQiniuMarketCatalogToPricing(pricing []model.Pricing, snapshot QiniuMarketCatalogSnapshot) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(snapshot.Models) == 0 {
		if snapshot.Strict {
			return []model.Pricing{}
		}
		return pricing
	}
	marketByID := make(map[string]dto.QiniuMarketModel, len(snapshot.Models))
	for _, item := range snapshot.Models {
		if strings.TrimSpace(item.ID) != "" {
			marketByID[item.ID] = item
		}
	}
	if len(marketByID) == 0 {
		if snapshot.Strict {
			return []model.Pricing{}
		}
		return pricing
	}

	enriched := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		marketModel, ok := marketByID[item.ModelName]
		if !ok {
			if !snapshot.Strict {
				enriched = append(enriched, item)
			}
			continue
		}
		item.Enabled = true
		item.Routable = len(item.EnableGroup) > 0
		item.PriceSource = QiniuMarketPriceSource
		item.PriceSourceLabel = QiniuMarketPriceSourceLabel
		marketCopy := marketModel
		item.QiniuMarket = &marketCopy
		applyQiniuMarketDisplayFields(&item, marketModel)
		enriched = append(enriched, item)
	}
	return enriched
}

func applyQiniuMarketDisplayFields(pricing *model.Pricing, marketModel dto.QiniuMarketModel) {
	if pricing == nil {
		return
	}
	localTags := pricing.Tags
	if strings.TrimSpace(marketModel.Description) != "" {
		pricing.Description = marketModel.Description
	}
	if strings.TrimSpace(marketModel.Avatar) != "" {
		pricing.Icon = marketModel.Avatar
	}
	pricing.Tags = joinQiniuMarketTags(marketModel.HotTags, marketModel.Features)
	if pricing.Tags == "" {
		pricing.Tags = strings.TrimSpace(localTags)
	}
	pricing.ContextLength = marketModel.ModelConstraints.ContextLength
	pricing.MaxOutputTokens = marketModel.ModelConstraints.MaxCompletionTokens
	if pricing.MaxOutputTokens <= 0 {
		pricing.MaxOutputTokens = marketModel.ModelConstraints.MaxDefaultCompletionTokens
	}
	if strings.TrimSpace(marketModel.ReleaseAt) != "" {
		pricing.ReleaseDate = marketModel.ReleaseAt
	} else {
		pricing.ReleaseDate = marketModel.CreatedTime
	}
	pricing.InputModalities = cloneStringSlice(marketModel.Architecture.InputModalities)
	pricing.OutputModalities = cloneStringSlice(marketModel.Architecture.OutputModalities)
	pricing.Capabilities = qiniuMarketCapabilities(marketModel)
}

func joinQiniuMarketTags(groups ...[]string) string {
	seen := make(map[string]struct{})
	tags := make([]string, 0)
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			tags = append(tags, value)
		}
	}
	return strings.Join(tags, ",")
}

func qiniuMarketCapabilities(marketModel dto.QiniuMarketModel) []string {
	capabilities := make([]string, 0)
	if marketModel.Architecture.FunctionCalling.Supported {
		capabilities = append(capabilities, "function_calling", "tools")
	}
	if marketModel.Architecture.SchemaOutput.Supported {
		capabilities = append(capabilities, "structured_output", "json_mode")
	}
	if marketModel.Architecture.Reasoning.Supported {
		capabilities = append(capabilities, "reasoning")
	}
	if marketModel.Architecture.ContentCache.Supported {
		capabilities = append(capabilities, "caching")
	}
	for _, feature := range marketModel.Features {
		if strings.Contains(feature, "工具") && !stringSliceContains(capabilities, "tools") {
			capabilities = append(capabilities, "tools")
		}
	}
	return capabilities
}

func QiniuMarketCatalogPublicState(snapshot QiniuMarketCatalogSnapshot) QiniuMarketCatalogState {
	state := QiniuMarketCatalogState{
		Status:        snapshot.Status,
		Stale:         snapshot.Stale,
		FromCache:     snapshot.FromCache,
		FallbackLocal: snapshot.FallbackLocal,
	}
	if !snapshot.LastSuccessTime.IsZero() {
		state.LastSuccessTime = snapshot.LastSuccessTime.Unix()
	}
	return state
}

func sanitizeQiniuMarketError(err error) string {
	message := sanitizeQiniuTaskError(err)
	return qiniuMarketSecretPattern.ReplaceAllString(message, "$1=********")
}

func cloneQiniuMarketModels(models []dto.QiniuMarketModel) []dto.QiniuMarketModel {
	cloned := make([]dto.QiniuMarketModel, len(models))
	copy(cloned, models)
	return cloned
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func resetQiniuMarketCatalogCacheForTest() {
	qiniuMarketCatalogCache.mu.Lock()
	defer qiniuMarketCatalogCache.mu.Unlock()
	qiniuMarketCatalogCache.cacheKey = ""
	qiniuMarketCatalogCache.models = nil
	qiniuMarketCatalogCache.lastSuccessTime = time.Time{}
	qiniuMarketCatalogCache.expiresAt = time.Time{}
	qiniuMarketCatalogCache.lastError = ""
	qiniuMarketCatalogCache.fetcher = fetchQiniuMarketCatalogModels
}

func setQiniuMarketCatalogFetcherForTest(fetcher qiniuMarketCatalogFetcher) {
	qiniuMarketCatalogCache.mu.Lock()
	defer qiniuMarketCatalogCache.mu.Unlock()
	qiniuMarketCatalogCache.fetcher = fetcher
}
