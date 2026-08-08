-- +goose Up
-- Envelopes cannot weaken the withdrawal restriction of their parent plan.
UPDATE envelopes AS e
SET unboxed_balance_policy = 'locked_until_parent_ends',
    updated_at = NOW()
FROM spending_plans AS p
WHERE e.plan_id = p.id
  AND p.unboxed_balance_policy = 'locked_until_plan_ends'
  AND e.unboxed_balance_policy <> 'locked_until_parent_ends';

-- +goose Down
-- This data correction is intentionally irreversible: the prior policy of an
-- inherited row cannot be reconstructed safely. A down migration must not
-- silently make locked funds withdrawable.
SELECT 1;
