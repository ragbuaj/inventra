-- Revert 000050. Urutan terbalik: hibah permission, tabel anak, tabel induk,
-- enum, lalu skema.

DELETE FROM identity.role_permissions WHERE permission_key = 'guide.manage';

DROP TABLE IF EXISTS guide.guide_attachments;
DROP TABLE IF EXISTS guide.guide_modules;

DROP TYPE IF EXISTS shared.guide_attachment_kind;
DROP TYPE IF EXISTS shared.guide_status;

DROP SCHEMA IF EXISTS guide;
