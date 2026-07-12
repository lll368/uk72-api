package operation_setting

import "testing"

func TestQiniuKeySettingDefaultsCostDetailAutomaticSettlement(t *testing.T) {
	if qiniuKeySetting.CostDetailLookbackDays != 7 {
		t.Fatalf("expected default cost-detail lookback to be 7 days, got %d", qiniuKeySetting.CostDetailLookbackDays)
	}
	if !qiniuKeySetting.CostDetailAutoApplyEnabled {
		t.Fatalf("expected missing cost-detail auto apply config to default enabled")
	}
	if qiniuKeySetting.ChildAccountBindingEnabled {
		t.Fatalf("expected child account binding to default disabled")
	}
	if qiniuKeySetting.BaseURL != QiniuKeyDefaultBaseURL {
		t.Fatalf("expected qiniu key base URL to default %s, got %s", QiniuKeyDefaultBaseURL, qiniuKeySetting.BaseURL)
	}
	if qiniuKeySetting.ChildAccountBaseURL != QiniuChildAccountDefaultBaseURL {
		t.Fatalf("expected qiniu child account base URL to default %s, got %s", QiniuChildAccountDefaultBaseURL, qiniuKeySetting.ChildAccountBaseURL)
	}
	if qiniuKeySetting.ChildAccountAssignmentMode != QiniuChildAccountAssignmentModeParentOnly {
		t.Fatalf("expected default child account assignment mode to be parent_only, got %s", qiniuKeySetting.ChildAccountAssignmentMode)
	}
	if qiniuKeySetting.ChildAccountBindingCutoverTime != 0 {
		t.Fatalf("expected child account binding cutover to default 0, got %d", qiniuKeySetting.ChildAccountBindingCutoverTime)
	}
}

func TestValidateQiniuChildAccountSettingForCreateUsesChildAccountBaseURL(t *testing.T) {
	setting := QiniuKeySetting{
		Enabled:                    true,
		BaseURL:                    QiniuKeyDefaultBaseURL,
		ChildAccountBaseURL:        "ftp://api.qiniu.com",
		AccessKey:                  "parent-ak",
		SecretKey:                  "parent-sk",
		ChildAccountEmailDomain:    "uk72.cn",
		ChildAccountEmailPrefix:    "child",
		ChildAccountPasswordLength: 18,
	}

	if err := ValidateQiniuChildAccountSettingForCreate(&setting); err == nil {
		t.Fatalf("expected invalid child account base URL to be rejected")
	}

	setting.ChildAccountBaseURL = " https://api.qiniu.com/ "
	if err := ValidateQiniuChildAccountSettingForCreate(&setting); err != nil {
		t.Fatalf("expected valid child account base URL to pass, got %v", err)
	}
	if setting.ChildAccountBaseURL != QiniuChildAccountDefaultBaseURL {
		t.Fatalf("expected child account base URL to be normalized, got %s", setting.ChildAccountBaseURL)
	}
}

func TestQiniuKeySettingPreservesExplicitCostDetailAutoApplyDisabled(t *testing.T) {
	setting := QiniuKeySetting{
		CostDetailAutoApplyEnabled: false,
	}

	normalizeQiniuKeySetting(&setting)

	if setting.CostDetailAutoApplyEnabled {
		t.Fatalf("expected explicit false cost-detail auto apply config to remain disabled")
	}
}

func TestQiniuKeySettingChildAccountBindingEligibilityUsesStrictCutover(t *testing.T) {
	setting := QiniuKeySetting{
		ChildAccountBindingEnabled:     true,
		ChildAccountAssignmentMode:     QiniuChildAccountAssignmentModeOneKeyOneChild,
		ChildAccountBindingCutoverTime: 100,
	}

	if IsQiniuChildAccountBindingEligible(&setting, 100) {
		t.Fatalf("expected same-second cutover user to stay on parent account")
	}
	if !IsQiniuChildAccountBindingEligible(&setting, 101) {
		t.Fatalf("expected post-cutover user to be eligible for child account binding")
	}

	setting.ChildAccountAssignmentMode = QiniuChildAccountAssignmentModeParentOnly
	if IsQiniuChildAccountBindingEligible(&setting, 101) {
		t.Fatalf("expected parent_only mode to keep user on parent account")
	}
}
