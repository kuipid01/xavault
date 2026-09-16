package pricing

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrQuoteInvalid = errors.New("fee quote is invalid or expired")

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func percentageFee(amount, bps int64) int64 {
	return int64(math.Ceil(float64(amount*bps) / 10000.0))
}

func feeFor(rule FeeRule, amount int64) int64 {
	fixed := rule.FixedFeeKobo
	if rule.FeeWaiverThresholdKobo != nil && amount < *rule.FeeWaiverThresholdKobo {
		fixed = 0
	}
	fee := percentageFee(amount, rule.PercentageBasisPoints) + fixed
	if rule.MaximumFeeKobo != nil && fee > *rule.MaximumFeeKobo {
		fee = *rule.MaximumFeeKobo
	}
	return fee
}

func withdrawalAmounts(principal, providerFee, levy int64, allowance bool) (userFee, subsidy, totalDeduction, recipientAmount int64, err error) {
	external := providerFee + levy
	userFee = external
	if allowance {
		userFee = 0
		subsidy = external
	}
	recipientAmount = principal - userFee
	if recipientAmount <= 0 {
		return 0, 0, 0, 0, errors.New("withdrawal amount must exceed applicable fees")
	}
	return userFee, subsidy, principal, recipientAmount, nil
}

func (s *Service) rule(operation, channel string, amount int64) (*FeeRule, error) {
	now := time.Now()
	var rule FeeRule
	err := s.db.Where("provider = ? AND operation = ? AND channel = ? AND currency = 'NGN' AND enabled = true AND effective_from <= ? AND (effective_until IS NULL OR effective_until > ?)", "paystack", operation, channel, now, now).
		Where("minimum_amount_kobo <= ? AND (maximum_amount_kobo IS NULL OR maximum_amount_kobo >= ?)", amount, amount).
		Order("effective_from DESC").First(&rule).Error
	return &rule, err
}

func (s *Service) CreateDepositQuote(userID string, walletCredit int64) (*Quote, error) {
	rule, err := s.rule(OperationDeposit, "checkout", walletCredit)
	if err != nil {
		return nil, err
	}
	gross := walletCredit
	for i := 0; i < 8; i++ {
		gross = walletCredit + feeFor(*rule, gross)
	}
	fee := gross - walletCredit
	q := &Quote{ID: uuid.NewString(), UserID: userID, Operation: OperationDeposit, Provider: "paystack", Channel: "checkout", Currency: "NGN", PrincipalAmount: walletCredit, ProviderFee: fee, GrossCharge: gross, NetCredit: walletCredit, TotalDeduction: gross, RecipientAmount: walletCredit, PricingVersion: rule.Version, Status: StatusActive, ContextJSON: "{}", ExpiresAt: time.Now().Add(15 * time.Minute), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return q, s.db.Create(q).Error
}

func (s *Service) DedicatedAccountNet(gross int64) (net, fee int64, version string, err error) {
	rule, err := s.rule(OperationDeposit, "dedicated_account", gross)
	if err != nil {
		return 0, 0, "", err
	}
	fee = feeFor(*rule, gross)
	net = gross - fee
	if net < 0 {
		net = 0
	}
	return net, fee, rule.Version, nil
}

func (s *Service) CreateWithdrawalQuote(userID string, principal int64, allowance bool, contextJSON string) (*Quote, error) {
	channel := "transfer_0_5000"
	if principal > 5_000_00 {
		channel = "transfer_5001_50000"
	}
	if principal > 50_000_00 {
		channel = "transfer_50001_plus"
	}
	rule, err := s.rule(OperationWithdrawal, channel, principal)
	if err != nil {
		return nil, err
	}
	providerFee := feeFor(*rule, principal)
	levy := int64(0)
	if rule.GovernmentLevyThresholdKobo != nil && principal >= *rule.GovernmentLevyThresholdKobo {
		levy = rule.GovernmentLevyKobo
	}
	userFee, subsidy, totalDeduction, recipientAmount, err := withdrawalAmounts(principal, providerFee, levy, allowance)
	if err != nil {
		return nil, err
	}
	q := &Quote{ID: uuid.NewString(), UserID: userID, Operation: OperationWithdrawal, Provider: "paystack", Channel: channel, Currency: "NGN", PrincipalAmount: principal, ProviderFee: providerFee, GovernmentLevy: levy, XavaultSubsidy: subsidy, UserFee: userFee, GrossCharge: principal, NetCredit: recipientAmount, TotalDeduction: totalDeduction, RecipientAmount: recipientAmount, AllowanceApplied: allowance, PricingVersion: rule.Version, Status: StatusActive, ContextJSON: contextJSON, ExpiresAt: time.Now().Add(15 * time.Minute), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return q, s.db.Create(q).Error
}

func (s *Service) ConsumeTx(tx *gorm.DB, quoteID, userID, operation string) (*Quote, error) {
	var q Quote
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND user_id=? AND operation=?", quoteID, userID, operation).First(&q).Error; err != nil {
		return nil, err
	}
	if q.Status != StatusActive || time.Now().After(q.ExpiresAt) {
		return nil, ErrQuoteInvalid
	}
	now := time.Now()
	q.Status = StatusConsumed
	q.ConsumedAt = &now
	q.UpdatedAt = now
	if err := tx.Save(&q).Error; err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Service) Consume(quoteID, userID, operation string) (*Quote, error) {
	var quote *Quote
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		quote, err = s.ConsumeTx(tx, quoteID, userID, operation)
		return err
	})
	return quote, err
}
