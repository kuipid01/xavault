package seed

import (
	"log"

	"github.com/kuipid01/salary-vault/internal/subscription"
	"gorm.io/gorm"
)

func SeedPlans(db *gorm.DB) error {
	plans := []subscription.SubscriptionPlan{
		{
			Code:                            subscription.PlanFree,
			Name:                            "Free",
			PriceKobo:                       0,
			MaxMonthlyVolumeKobo:            5_000_000, // ₦50k
			MaxLockedPlans:                  2,
			MaxFlexibleEnvelopes:            2,
			MaxEmergencyVaults:              1,
			FreeFlexibleWithdrawalsMonthly:  0,
			FreeEmergencyWithdrawalsMonthly: 0,
		},
		{
			Code:                            subscription.PlanLite,
			Name:                            "Lite",
			PriceKobo:                       50_000,     // ₦500
			MaxMonthlyVolumeKobo:            15_000_000, // ₦150k
			MaxLockedPlans:                  5,
			MaxFlexibleEnvelopes:            5,
			MaxEmergencyVaults:              1,
			FreeFlexibleWithdrawalsMonthly:  1,
			FreeEmergencyWithdrawalsMonthly: 1,
		},
		{
			Code:                            subscription.PlanPremium,
			Name:                            "Premium",
			PriceKobo:                       100_000,    // ₦1,000
			MaxMonthlyVolumeKobo:            50_000_000, // ₦500k
			MaxLockedPlans:                  15,
			MaxFlexibleEnvelopes:            15,
			MaxEmergencyVaults:              1,
			FreeFlexibleWithdrawalsMonthly:  3,
			FreeEmergencyWithdrawalsMonthly: 1,
			HasSalarySplit:                  true,
			HasFullAnalytics:                true,
		},
		{
			Code:                            subscription.PlanPower,
			Name:                            "Power",
			PriceKobo:                       200_000, // ₦2,000
			MaxMonthlyVolumeKobo:            -1,      // Unlimited
			MaxLockedPlans:                  -1,
			MaxFlexibleEnvelopes:            -1,
			MaxEmergencyVaults:              3,
			FreeFlexibleWithdrawalsMonthly:  8,
			FreeEmergencyWithdrawalsMonthly: 1,
			HasSalarySplit:                  true,
			HasFullAnalytics:                true,
			HasSharedEnvelopes:              true,
			HasAdvancedRules:                true,
		},
	}

	for _, p := range plans {
		if p.Currency == "" {
			p.Currency = "NGN"
		}

		if err := db.
			Where("code = ?", p.Code).
			Attrs(subscription.SubscriptionPlan{
				Code: p.Code,
			}).
			Assign(map[string]interface{}{
				"name":                               p.Name,
				"price_kobo":                         p.PriceKobo,
				"currency":                           p.Currency,
				"max_monthly_volume_kobo":            p.MaxMonthlyVolumeKobo,
				"max_locked_plans":                   p.MaxLockedPlans,
				"max_flexible_envelopes":             p.MaxFlexibleEnvelopes,
				"max_emergency_vaults":               p.MaxEmergencyVaults,
				"free_flexible_withdrawals_monthly":  p.FreeFlexibleWithdrawalsMonthly,
				"free_emergency_withdrawals_monthly": p.FreeEmergencyWithdrawalsMonthly,
				"has_salary_split":                   p.HasSalarySplit,
				"has_full_analytics":                 p.HasFullAnalytics,
				"has_shared_envelopes":               p.HasSharedEnvelopes,
				"has_advanced_rules":                 p.HasAdvancedRules,
			}).
			FirstOrCreate(&subscription.SubscriptionPlan{}).Error; err != nil {
			return err
		}
	}

	return nil
}
func Init(db *gorm.DB) {

	if err := SeedPlans(db); err != nil {
		log.Fatal(err)
	}
}
