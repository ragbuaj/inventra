-- Transfer (mutasi) lifecycle notification types.
-- Adds the four transfer-stage notification kinds fanned out by the notification
-- consumer: origin office is told a transfer is approved/received/returned, and
-- the destination office is told an asset is incoming (in_transit).
--
-- ALTER TYPE ... ADD VALUE is safe here: PostgreSQL 12+ allows it inside the
-- transaction golang-migrate wraps each migration in, as long as the new value
-- is not USED in that same transaction (it is not — only added). Mirrors the
-- pattern in 000022 (transfer_status ADD VALUE 'returned').
ALTER TYPE shared.notification_type ADD VALUE IF NOT EXISTS 'transfer_approved';
ALTER TYPE shared.notification_type ADD VALUE IF NOT EXISTS 'transfer_in_transit';
ALTER TYPE shared.notification_type ADD VALUE IF NOT EXISTS 'transfer_received';
ALTER TYPE shared.notification_type ADD VALUE IF NOT EXISTS 'transfer_returned';
