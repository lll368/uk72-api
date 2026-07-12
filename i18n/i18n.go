package i18n

import (
	"embed"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

const (
	LangZhCN    = "zh-CN"
	LangZhTW    = "zh-TW"
	LangEn      = "en"
	LangKo      = "ko"
	LangAr      = "ar"
	LangPtBR    = "pt-BR"
	LangEsES    = "es-ES"
	DefaultLang = LangEn // Fallback to English if language not supported
)

//go:embed locales/*.yaml
var localeFS embed.FS

var (
	bundle     *i18n.Bundle
	localizers = make(map[string]*i18n.Localizer)
	mu         sync.RWMutex
	initOnce   sync.Once
)

var supportedLanguages = []string{LangZhCN, LangZhTW, LangEn, LangKo, LangAr, LangPtBR, LangEsES}

// Init initializes the i18n bundle and loads all translation files
func Init() error {
	var initErr error
	initOnce.Do(func() {
		bundle = i18n.NewBundle(language.Chinese)
		bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

		// Load embedded translation files
		files := []string{
			"locales/zh-CN.yaml",
			"locales/zh-TW.yaml",
			"locales/en.yaml",
			"locales/ko.yaml",
			"locales/ar.yaml",
			"locales/pt-BR.yaml",
			"locales/es-ES.yaml",
		}
		for _, file := range files {
			_, err := bundle.LoadMessageFileFS(localeFS, file)
			if err != nil {
				initErr = err
				return
			}
		}

		// Pre-create localizers for supported languages
		localizers[LangZhCN] = i18n.NewLocalizer(bundle, LangZhCN)
		localizers[LangZhTW] = i18n.NewLocalizer(bundle, LangZhTW)
		localizers[LangEn] = i18n.NewLocalizer(bundle, LangEn)
		localizers[LangKo] = i18n.NewLocalizer(bundle, LangKo, LangEn)
		localizers[LangAr] = i18n.NewLocalizer(bundle, LangAr, LangEn)
		localizers[LangPtBR] = i18n.NewLocalizer(bundle, LangPtBR, LangEn)
		localizers[LangEsES] = i18n.NewLocalizer(bundle, LangEsES, LangEn)

		// Set the TranslateMessage function in common package
		common.TranslateMessage = T
	})
	return initErr
}

// GetLocalizer returns a localizer for the specified language
func GetLocalizer(lang string) *i18n.Localizer {
	lang = normalizeLang(lang)

	mu.RLock()
	loc, ok := localizers[lang]
	mu.RUnlock()

	if ok {
		return loc
	}

	// Create new localizer for unknown language (fallback to default)
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring write lock
	if loc, ok = localizers[lang]; ok {
		return loc
	}

	loc = i18n.NewLocalizer(bundle, lang, DefaultLang)
	localizers[lang] = loc
	return loc
}

// T translates a message key using the language from gin context
func T(c *gin.Context, key string, args ...map[string]any) string {
	lang := GetLangFromContext(c)
	return Translate(lang, key, args...)
}

// Translate translates a message key for the specified language
func Translate(lang, key string, args ...map[string]any) string {
	loc := GetLocalizer(lang)

	config := &i18n.LocalizeConfig{
		MessageID: key,
	}

	if len(args) > 0 && args[0] != nil {
		config.TemplateData = args[0]
	}

	msg, err := loc.Localize(config)
	if err != nil {
		// Return key as fallback if translation not found
		return key
	}
	return msg
}

// userLangLoaderFunc is a function that loads user language from database/cache
// It's set by the model package to avoid circular imports
var userLangLoaderFunc func(userId int) string

// SetUserLangLoader sets the function to load user language (called from model package)
func SetUserLangLoader(loader func(userId int) string) {
	userLangLoaderFunc = loader
}

// GetLangFromContext extracts the language setting from gin context
// It checks multiple sources in priority order:
// 1. User settings (ContextKeyUserSetting) - if already loaded (e.g., by TokenAuth)
// 2. Lazy load user language from cache/DB using user ID
// 3. Language set by middleware (ContextKeyLanguage) - from Accept-Language header
// 4. Default language (English)
func GetLangFromContext(c *gin.Context) string {
	if c == nil {
		return DefaultLang
	}

	// 1. Try to get language from user settings (if already loaded by TokenAuth or other middleware)
	if userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting); ok {
		if lang, ok := normalizeSupportedLang(userSetting.Language); ok {
			return lang
		}
	}

	// 2. Lazy load user language using user ID (for session-based auth where full settings aren't loaded)
	if userLangLoaderFunc != nil {
		if userId, exists := c.Get("id"); exists {
			if uid, ok := userId.(int); ok && uid > 0 {
				lang := userLangLoaderFunc(uid)
				if normalized, ok := normalizeSupportedLang(lang); ok {
					return normalized
				}
			}
		}
	}

	// 3. Try to get language from context (set by I18n middleware from Accept-Language)
	if lang := c.GetString(string(constant.ContextKeyLanguage)); lang != "" {
		if normalized, ok := normalizeSupportedLang(lang); ok {
			return normalized
		}
	}

	// 4. Try Accept-Language header directly (fallback if middleware didn't run)
	if acceptLang := c.GetHeader("Accept-Language"); acceptLang != "" {
		lang := ParseAcceptLanguage(acceptLang)
		if IsSupported(lang) {
			return lang
		}
	}

	return DefaultLang
}

// ParseAcceptLanguage parses the Accept-Language header and returns the preferred language
func ParseAcceptLanguage(header string) string {
	if header == "" {
		return DefaultLang
	}

	// Simple parsing: take the first language tag
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return DefaultLang
	}

	// Get the first language and remove quality value
	firstLang := strings.TrimSpace(parts[0])
	if idx := strings.Index(firstLang, ";"); idx > 0 {
		firstLang = firstLang[:idx]
	}

	return normalizeLang(firstLang)
}

// NormalizeLang normalizes language code to the canonical supported format.
func NormalizeLang(lang string) string {
	return normalizeLang(lang)
}

// NormalizeSupportedLang normalizes a language code and reports whether it is supported.
func NormalizeSupportedLang(lang string) (string, bool) {
	return normalizeSupportedLang(lang)
}

// normalizeLang normalizes language code to supported format
func normalizeLang(lang string) string {
	if normalized, ok := normalizeSupportedLang(lang); ok {
		return normalized
	}
	return DefaultLang
}

func normalizeSupportedLang(lang string) (string, bool) {
	lang = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(lang, "_", "-")))

	// Handle common variations
	switch {
	case hasLanguageTagPrefix(lang, "zh-tw") || hasLanguageTagPrefix(lang, "zh-hk") || hasLanguageTagPrefix(lang, "zh-mo") || hasLanguageTagPrefix(lang, "zh-hant"):
		return LangZhTW, true
	case hasLanguageTagPrefix(lang, "zh"):
		return LangZhCN, true
	case hasLanguageTagPrefix(lang, "ko"):
		return LangKo, true
	case hasLanguageTagPrefix(lang, "ar"):
		return LangAr, true
	case hasLanguageTagPrefix(lang, "pt"):
		return LangPtBR, true
	case hasLanguageTagPrefix(lang, "es"):
		return LangEsES, true
	case hasLanguageTagPrefix(lang, "en"):
		return LangEn, true
	default:
		return "", false
	}
}

func hasLanguageTagPrefix(lang string, prefix string) bool {
	return lang == prefix || strings.HasPrefix(lang, prefix+"-")
}

// SupportedLanguages returns a list of supported language codes
func SupportedLanguages() []string {
	return append([]string(nil), supportedLanguages...)
}

// IsSupported checks if a language code is supported
func IsSupported(lang string) bool {
	_, ok := normalizeSupportedLang(lang)
	return ok
}
