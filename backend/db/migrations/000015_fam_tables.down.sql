-- Reverse 000015: drop BAST/documents table then the new schemas (CASCADE).
DROP TABLE IF EXISTS asset.asset_documents CASCADE;
DROP SCHEMA IF EXISTS stockopname CASCADE;
DROP SCHEMA IF EXISTS disposal CASCADE;
DROP SCHEMA IF EXISTS transfer CASCADE;
