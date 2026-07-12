package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.POST("/contact/messages", middleware.CriticalRateLimit(), controller.SubmitContactMessage)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), controller.GetPricing)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", controller.GetPerfMetricsSummary)
			perfMetricsRoute.GET("", controller.GetPerfMetrics)
		}
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), controller.GetRankings)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.POST("/phone/verification", middleware.PhoneVerificationRateLimit(), middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPhoneVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.ResetPassword)
		apiRouter.POST("/user/reset/verify", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.VerifyPasswordReset)
		apiRouter.POST("/user/reset/confirm", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.ConfirmPasswordReset)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.CriticalRateLimit(), controller.EmailBind)
		// Non-standard OAuth (WeChat, Telegram) - keep original routes
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.CriticalRateLimit(), controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), controller.TelegramLogin)
		apiRouter.GET("/oauth/telegram/bind", middleware.CriticalRateLimit(), controller.TelegramBind)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)
		apiRouter.GET("/all-pricing", controller.GetAllPricing)

		apiRouter.POST("/stripe/webhook", controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", controller.WaffoWebhook)
		apiRouter.POST("/alipay/notify", controller.AlipayNotify)
		apiRouter.POST("/wechat/notify", controller.WechatPayNotify)
		apiRouter.POST("/withdraw/piggy/contract/notify", controller.PiggyContractNotify)
		apiRouter.POST("/withdraw/piggy/payment/notify", controller.PiggyPaymentNotify)
		//apiRouter.POST("/waffo-pancake/webhook", controller.WaffoPancakeWebhook)
		apiRouter.POST("/vip/epay/notify", controller.VipActivationEpayNotify)
		apiRouter.GET("/vip/epay/notify", controller.VipActivationEpayNotify)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.UniversalVerify)

		vipRoute := apiRouter.Group("/vip")
		vipRoute.Use(middleware.UserAuth())
		{
			vipRoute.GET("/info", controller.GetVipActivationInfo)
			vipRoute.GET("/orders/:trade_no/status", controller.GetVipActivationOrderStatus)
			vipRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.VipActivationRequestEpay)
			vipRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.VipActivationRequestStripe)
			vipRoute.POST("/alipay/pay", middleware.CriticalRateLimit(), controller.VipActivationRequestAlipay)
			vipRoute.POST("/wechat/pay", middleware.CriticalRateLimit(), controller.VipActivationRequestWechatPay)
			vipRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.VipActivationRequestCreem)
			vipRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.VipActivationRequestWaffo)
			vipRoute.GET("/subordinates", controller.ListVipSubordinates)
			vipRoute.PUT("/subordinates/:child_user_id/discount", middleware.CriticalRateLimit(), controller.UpdateVipSubordinateDiscount)
			vipRoute.DELETE("/subordinates/:child_user_id/discount", middleware.CriticalRateLimit(), controller.ResetVipSubordinateDiscount)
		}
		vipAdminRoute := apiRouter.Group("/vip/admin")
		vipAdminRoute.Use(middleware.AdminAuth())
		{
			vipAdminRoute.GET("/records", controller.AdminListVipActivationRecords)
			vipAdminRoute.POST("/users/:id/disable", controller.AdminDisableVipActivation)
			vipAdminRoute.POST("/users/:id/activate", controller.AdminManualActivateVvip)
			vipAdminRoute.GET("/relations", controller.AdminListUserRelations)
			vipAdminRoute.POST("/relations", controller.AdminCreateUserRelation)
			vipAdminRoute.POST("/relations/:id/disable", controller.AdminDisableUserRelation)
		}
		walletRoute := apiRouter.Group("/wallet")
		walletRoute.Use(middleware.UserAuth())
		{
			walletRoute.GET("/account", controller.GetWalletAccount)
			walletRoute.GET("/flows", controller.GetWalletFlows)
			walletRoute.GET("/commissions", controller.GetWalletCommissions)
			walletRoute.POST("/commission/transfer", middleware.CriticalRateLimit(), controller.TransferWalletCommission)
			walletRoute.GET("/withdraw/profile", controller.GetWalletWithdrawalProfile)
			walletRoute.PUT("/withdraw/profile", middleware.CriticalRateLimit(), controller.SaveWalletWithdrawalProfile)
			walletRoute.GET("/withdraw/eligibility", controller.GetWalletWithdrawalEligibility)
			walletRoute.POST("/withdraw/piggy/sign-url", middleware.CriticalRateLimit(), controller.GetWalletPiggySignURL)
			walletRoute.POST("/withdraw/piggy/sign-status/refresh", middleware.CriticalRateLimit(), controller.RefreshWalletPiggyContractStatus)
			walletRoute.POST("/withdraw/piggy/contract-preview", middleware.CriticalRateLimit(), controller.GetWalletPiggyContractPreview)
			walletRoute.POST("/withdraw/piggy/tax-trial", middleware.PiggyTaxTrialRateLimit(), controller.TrialWalletPiggyWithdrawTax)
			walletRoute.POST("/withdraw/piggy", middleware.CriticalRateLimit(), controller.SubmitWalletPiggyWithdraw)
			// walletRoute.POST("/withdraw", middleware.CriticalRateLimit(), controller.SubmitWalletWithdraw)
			walletRoute.GET("/withdraws", controller.GetWalletWithdraws)
		}
		walletAdminRoute := apiRouter.Group("/wallet/admin")
		walletAdminRoute.Use(middleware.AdminAuth())
		{
			walletAdminRoute.GET("/commissions", controller.AdminListWalletCommissions)
			walletAdminRoute.GET("/withdraws", controller.AdminListWalletWithdraws)
			walletAdminRoute.GET("/withdraw-callbacks", controller.AdminListPiggyWithdrawCallbacks)
			walletAdminRoute.POST("/withdraws/:id/piggy/manual-result", controller.AdminRecordPiggyWithdrawManualResult)
			walletAdminRoute.POST("/withdraws/:id/piggy/retry-confirm", controller.AdminRetryPiggyWithdrawConfirm)
			walletAdminRoute.POST("/withdraws/:id/piggy/recover-submit", controller.AdminRecoverPiggyWithdrawSubmit)
			walletAdminRoute.POST("/withdraws/:id/piggy/cancel", controller.AdminCancelPiggyWithdraw)
			walletAdminRoute.POST("/piggy/withdraws/scan", controller.AdminScanPiggyWithdrawCompensations)
			walletAdminRoute.POST("/withdraws/:id/approve", controller.AdminApproveWalletWithdraw)
			walletAdminRoute.POST("/withdraws/:id/reject", controller.AdminRejectWalletWithdraw)
			walletAdminRoute.POST("/withdraws/:id/pay", controller.AdminPayWalletWithdraw)
			walletAdminRoute.POST("/withdraws/:id/fail", controller.AdminFailWalletWithdraw)
		}
		paymentAdminRoute := apiRouter.Group("/payment/admin")
		paymentAdminRoute.Use(middleware.AdminAuth())
		{
			paymentAdminRoute.GET("/callback-logs", controller.AdminListPaymentCallbackLogs)
			paymentAdminRoute.POST("/reconciliation/tasks", controller.AdminCreatePaymentReconciliationTask)
			paymentAdminRoute.GET("/reconciliation/tasks", controller.AdminListPaymentReconciliationTasks)
			paymentAdminRoute.GET("/qiniu-key-tasks", controller.AdminListQiniuKeyTasks)
			paymentAdminRoute.POST("/qiniu-key-tasks/scan", controller.AdminScanQiniuKeyTasks)
			paymentAdminRoute.POST("/qiniu-key-tasks/:id/retry", controller.AdminRetryQiniuKeyTask)
			paymentAdminRoute.GET("/qiniu-keys", controller.AdminListQiniuKeys)
			paymentAdminRoute.POST("/qiniu-keys/:id/disable", controller.AdminDisableQiniuKey)
			paymentAdminRoute.GET("/qiniu-child-accounts", controller.AdminListQiniuChildAccounts)
			paymentAdminRoute.POST("/qiniu-child-accounts", controller.AdminCreateQiniuChildAccount)
			paymentAdminRoute.GET("/qiniu-child-accounts/:id", controller.AdminGetQiniuChildAccount)
			paymentAdminRoute.POST("/qiniu-child-accounts/:id/disable", controller.AdminDisableQiniuChildAccount)
			paymentAdminRoute.POST("/qiniu-child-accounts/:id/enable", controller.AdminEnableQiniuChildAccount)
			paymentAdminRoute.GET("/qiniu-child-account-tasks", controller.AdminListQiniuChildAccountTasks)
			paymentAdminRoute.POST("/qiniu-child-account-tasks/:id/retry", controller.AdminRetryQiniuChildAccountTask)
			paymentAdminRoute.GET("/qiniu-official-records", controller.AdminListQiniuOfficialUsageRecords)
			paymentAdminRoute.POST("/qiniu-official-records/:id/retry", controller.AdminRetryQiniuOfficialUsageRecord)
			paymentAdminRoute.GET("/qiniu-official-ledger-applications", controller.AdminListQiniuOfficialLedgerApplications)
			paymentAdminRoute.POST("/qiniu-official-ledger-applications/:id/retry", controller.AdminRetryQiniuOfficialLedgerApplication)
			paymentAdminRoute.GET("/qiniu-billing-summary", controller.AdminGetQiniuBillingSummary)
			paymentAdminRoute.GET("/qiniu-billing-buckets", controller.AdminListQiniuBillingBuckets)
			paymentAdminRoute.POST("/qiniu-billing-buckets/:id/recalculate", controller.AdminRecalculateQiniuBillingBucket)
			paymentAdminRoute.POST("/qiniu-billing-buckets/:id/resolve", controller.AdminResolveQiniuBillingBucket)
			paymentAdminRoute.POST("/qiniu-billing-buckets/:id/skip", controller.AdminSkipQiniuBillingBucket)
			paymentAdminRoute.GET("/qiniu-cost-detail-records", controller.AdminListQiniuCostDetailRecords)
			paymentAdminRoute.POST("/qiniu-cost-detail-records/:id/resolve", controller.AdminResolveQiniuCostDetailRecord)
			paymentAdminRoute.GET("/qiniu-billing-bucket-items", controller.AdminListQiniuBillingBucketItems)
			paymentAdminRoute.GET("/qiniu-billing-bucket-applications", controller.AdminListQiniuBillingBucketApplications)
			paymentAdminRoute.POST("/qiniu-billing-bucket-applications/:id/retry", controller.AdminRetryQiniuBillingBucketApplication)
			paymentAdminRoute.POST("/topups/:trade_no/reverse", controller.AdminReverseTopUpOrder)
			paymentAdminRoute.POST("/vip-activations/:trade_no/reverse", controller.AdminReverseVipActivationOrder)
			paymentAdminRoute.POST("/vip-activations/:trade_no/repair", controller.AdminRepairVipActivationSettlement)
		}
		contactAdminRoute := apiRouter.Group("/contact/admin")
		contactAdminRoute.Use(middleware.AdminAuth())
		{
			contactAdminRoute.GET("/messages", controller.AdminListContactMessages)
			contactAdminRoute.PUT("/messages/:id", controller.AdminUpdateContactMessage)
			contactAdminRoute.DELETE("/messages/:id", controller.AdminDeleteContactMessage)
		}

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/register/phone", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.RegisterByPhone)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/email", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.LoginByEmail)
			userRoute.POST("/login/phone", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.LoginByPhone)
			userRoute.POST("/reset_password/phone", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.ResetPasswordByPhone)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), controller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.GET("/logout", controller.Logout)
			userRoute.POST("/epay/notify", controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/alipay/pay", middleware.CriticalRateLimit(), controller.RequestAlipayPay)
				selfRoute.POST("/alipay/amount", controller.RequestAlipayAmount)
				selfRoute.POST("/wechat/pay", middleware.CriticalRateLimit(), controller.RequestWechatPayPay)
				selfRoute.POST("/wechat/amount", controller.RequestWechatPayAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/amount", controller.RequestWaffoAmount)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				//selfRoute.POST("/waffo-pancake/amount", controller.RequestWaffoPancakeAmount)
				//selfRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPancakePay)
				selfRoute.POST("/aff_transfer", controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", controller.Setup2FA)
				selfRoute.POST("/2fa/enable", controller.Enable2FA)
				selfRoute.POST("/2fa/disable", controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/topup", controller.GetAllTopUps)
				adminRoute.POST("/topup/complete", controller.AdminCompleteTopUp)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id/oauth/bindings", controller.GetUserOAuthBindingsByAdmin)
				adminRoute.DELETE("/:id/oauth/bindings/:provider_id", controller.UnbindCustomOAuthByAdmin)
				adminRoute.DELETE("/:id/bindings/:binding_type", controller.AdminClearUserBinding)
				adminRoute.PUT("/:id/discount", controller.UpdateUserTopupDiscount)
				adminRoute.DELETE("/:id/discount", controller.ResetUserTopupDiscount)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", controller.AdminResetPasskey)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", controller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", controller.AdminDisable2FA)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			subscriptionAdminRoute.GET("/plans", controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.POST("/plans", controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.POST("/bind", controller.AdminBindSubscription)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", controller.SubscriptionEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.PUT("/batch", controller.UpdateOptions)
			optionRoute.POST("/payment_compliance", controller.ConfirmPaymentCompliance)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.POST("/migrate_console_setting", controller.MigrateConsoleSetting) // 用于迁移检测的旧键，下个版本会删除
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		{
			channelRoute.GET("/", controller.GetAllChannels)
			channelRoute.GET("/search", controller.SearchChannels)
			channelRoute.GET("/models", controller.ChannelListModels)
			channelRoute.GET("/models_enabled", controller.EnabledListModels)
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), controller.GetChannelKey)
			channelRoute.GET("/test", controller.TestAllChannels)
			channelRoute.GET("/test/:id", controller.TestChannel)
			channelRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", controller.UpdateChannelBalance)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/", controller.UpdateChannel)
			channelRoute.DELETE("/disabled", controller.DeleteDisabledChannel)
			channelRoute.POST("/tag/disabled", controller.DisableTagChannels)
			channelRoute.POST("/tag/enabled", controller.EnableTagChannels)
			channelRoute.PUT("/tag", controller.EditTagChannels)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.POST("/batch", controller.DeleteChannelBatch)
			channelRoute.POST("/fix", controller.FixChannelsAbilities)
			channelRoute.GET("/fetch_models/:id", controller.FetchUpstreamModels)
			channelRoute.POST("/fetch_models", middleware.RootAuth(), controller.FetchModels)
			channelRoute.POST("/codex/oauth/start", controller.StartCodexOAuth)
			channelRoute.POST("/codex/oauth/complete", controller.CompleteCodexOAuth)
			channelRoute.POST("/:id/codex/oauth/start", controller.StartCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/oauth/complete", controller.CompleteCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/refresh", controller.RefreshCodexChannelCredential)
			channelRoute.GET("/:id/codex/usage", controller.GetCodexChannelUsage)
			channelRoute.POST("/ollama/pull", controller.OllamaPullModel)
			channelRoute.POST("/ollama/pull/stream", controller.OllamaPullModelStream)
			channelRoute.DELETE("/ollama/delete", controller.OllamaDeleteModel)
			channelRoute.GET("/ollama/version/:id", controller.OllamaVersion)
			channelRoute.POST("/batch/tag", controller.BatchSetChannelTag)
			channelRoute.GET("/tag/models", controller.GetTagModels)
			channelRoute.POST("/copy/:id", controller.CopyChannel)
			channelRoute.POST("/multi_key/manage", controller.ManageMultiKeys)
			channelRoute.POST("/upstream_updates/apply", controller.ApplyChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/apply_all", controller.ApplyAllChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect", controller.DetectChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect_all", controller.DetectAllChannelUpstreamModelUpdates)
		}
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/qiniu/status", controller.GetQiniuKeyStatus)
			tokenRoute.POST("/qiniu/default/retry", middleware.CriticalRateLimit(), controller.RetryQiniuDefaultKey)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
		logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), controller.GetAllQuotaDates)
		dataRoute.GET("/users", middleware.AdminAuth(), controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("/", middleware.AdminAuth(), controller.GetAllTask)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", controller.SyncUpstreamModels)
			modelsRoute.GET("/missing", controller.GetMissingModels)
			modelsRoute.GET("/", controller.GetAllModelsMeta)
			modelsRoute.GET("/search", controller.SearchModelsMeta)
			modelsRoute.GET("/:id", controller.GetModelMeta)
			modelsRoute.POST("/", controller.CreateModelMeta)
			modelsRoute.PUT("/", controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}
	}
}
