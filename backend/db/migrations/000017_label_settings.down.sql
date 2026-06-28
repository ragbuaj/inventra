-- Remove label settings seeded in 000017_label_settings.up.sql.
DELETE FROM identity.app_settings
WHERE key IN ('label.company_name', 'label.disclaimer');
