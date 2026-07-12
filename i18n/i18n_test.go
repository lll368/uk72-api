package i18n

import (
	"slices"
	"testing"
)

func TestNormalizeLangForAddedLocales(t *testing.T) {
	tests := map[string]string{
		"zh":      LangZhCN,
		"zh_CN":   LangZhCN,
		"zh-Hans": LangZhCN,
		"zh-TW":   LangZhTW,
		"zh_HK":   LangZhTW,
		"ko":      LangKo,
		"ko-KR":   LangKo,
		"ar":      LangAr,
		"ar-SA":   LangAr,
	}

	for input, want := range tests {
		if got := NormalizeLang(input); got != want {
			t.Fatalf("NormalizeLang(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeLangDoesNotMatchUnsupportedPrefixes(t *testing.T) {
	tests := []string{
		"archive",
		"arbitrary",
		"koala",
		"korean",
		"english",
		"ptolemy",
		"esoteric",
		"zhfoobar",
	}

	for _, input := range tests {
		if got := NormalizeLang(input); got != DefaultLang {
			t.Fatalf("NormalizeLang(%q) = %q, want %q", input, got, DefaultLang)
		}
	}
}

func TestNormalizeSupportedLangRejectsUnsupportedPrefixes(t *testing.T) {
	tests := []string{
		"archive",
		"arbitrary",
		"koala",
		"korean",
		"english",
		"ptolemy",
		"esoteric",
		"zhfoobar",
	}

	for _, input := range tests {
		if got, ok := NormalizeSupportedLang(input); ok {
			t.Fatalf("NormalizeSupportedLang(%q) = %q, true; want unsupported", input, got)
		}
		if IsSupported(input) {
			t.Fatalf("IsSupported(%q) = true; want false", input)
		}
	}
}

func TestParseAcceptLanguageForAddedLocales(t *testing.T) {
	tests := map[string]string{
		"ko-KR,ko;q=0.9,en;q=0.8":   LangKo,
		"ar-SA,ar;q=0.9,en;q=0.8":   LangAr,
		"zh-Hant-TW,zh;q=0.9":       LangZhTW,
		"zh-Hans-CN,zh;q=0.9":       LangZhCN,
		"pt-BR,pt;q=0.9,en;q=0.8":   LangPtBR,
		"es-ES,es;q=0.9,en;q=0.8":   LangEsES,
		"en-US,en;q=0.9,ko;q=0.8":   LangEn,
		"arbitrary,en;q=0.9":        DefaultLang,
		"koala,en;q=0.9":            DefaultLang,
		"unsupported,en;q=0.9,ko;q": DefaultLang,
	}

	for input, want := range tests {
		if got := ParseAcceptLanguage(input); got != want {
			t.Fatalf("ParseAcceptLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSupportedLanguagesContainAddedLocales(t *testing.T) {
	supported := SupportedLanguages()
	for _, lang := range []string{LangKo, LangAr} {
		if !slices.Contains(supported, lang) {
			t.Fatalf("SupportedLanguages() missing %q", lang)
		}
	}
}

func TestTranslateLoadsAddedLocales(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	for _, lang := range []string{LangKo, LangAr} {
		if got := Translate(lang, "common.operation_success"); got == "common.operation_success" || got == "" {
			t.Fatalf("Translate(%q, common.operation_success) = %q", lang, got)
		}
	}
}
