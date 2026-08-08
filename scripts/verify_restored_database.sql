\set ON_ERROR_STOP on

DO $$
DECLARE missing_tables text;
BEGIN
  SELECT string_agg(required.name, ', ' ORDER BY required.name)
  INTO missing_tables
  FROM (VALUES
    ('users'), ('wallets'), ('wallet_deposits'), ('wallet_ledgers'),
    ('spending_plans'), ('withdrawals'), ('paystack_events'),
    ('reconciliation_controls'), ('goose_db_version')
  ) AS required(name)
  WHERE to_regclass('public.' || required.name) IS NULL;
  IF missing_tables IS NOT NULL THEN
    RAISE EXCEPTION 'restore is missing required tables: %', missing_tables;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM wallets
    WHERE available_balance < 0 OR locked_balance < 0
       OR ledger_balance <> available_balance + locked_balance
  ) THEN
    RAISE EXCEPTION 'wallet balance invariant failed after restore';
  END IF;
  IF EXISTS (SELECT reference FROM wallet_ledgers GROUP BY reference HAVING count(*) > 1) THEN
    RAISE EXCEPTION 'duplicate wallet ledger reference found after restore';
  END IF;
  IF EXISTS (SELECT reference FROM wallet_deposits GROUP BY reference HAVING count(*) > 1) THEN
    RAISE EXCEPTION 'duplicate deposit reference found after restore';
  END IF;
  IF EXISTS (SELECT reference FROM withdrawals GROUP BY reference HAVING count(*) > 1) THEN
    RAISE EXCEPTION 'duplicate withdrawal reference found after restore';
  END IF;
END $$;

SELECT 'users' AS entity, count(*) AS restored_rows FROM users
UNION ALL SELECT 'wallets', count(*) FROM wallets
UNION ALL SELECT 'wallet_deposits', count(*) FROM wallet_deposits
UNION ALL SELECT 'wallet_ledgers', count(*) FROM wallet_ledgers
UNION ALL SELECT 'withdrawals', count(*) FROM withdrawals
UNION ALL SELECT 'paystack_events', count(*) FROM paystack_events
ORDER BY entity;
