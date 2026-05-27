-- +migrate Up
ALTER TABLE `local_asr_config` ALTER COLUMN `enabled` SET DEFAULT 0;

-- +migrate Down
ALTER TABLE `local_asr_config` ALTER COLUMN `enabled` SET DEFAULT 1;
