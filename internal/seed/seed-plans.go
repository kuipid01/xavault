package seed

import (
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
			PriceKobo:                       200_000,     // ₦2,000
			MaxMonthlyVolumeKobo:            200_000_000, // ₦2m
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

	for _, plan := range plans {
		if err := db.Where("code = ?", plan.Code).
			Assign(plan).
			FirstOrCreate(&plan).Error; err != nil {
			return err
		}
	}

	return nil
}
