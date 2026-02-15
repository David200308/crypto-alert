CREATE DATABASE IF NOT EXISTS web3;
USE web3;

-- Token (price) alert rules
CREATE TABLE IF NOT EXISTS alert_rule_token_config (
  id               BIGINT AUTO_INCREMENT PRIMARY KEY,
  symbol           VARCHAR(64) NOT NULL,
  price_feed_id    VARCHAR(128) NOT NULL,
  threshold        DOUBLE NOT NULL,
  direction        VARCHAR(8) NOT NULL,
  enabled          BOOLEAN NOT NULL DEFAULT true,
  frequency        JSON,
  recipient_email  VARCHAR(255) NOT NULL
);

-- DeFi alert rules (params and frequency stored as JSON)
CREATE TABLE IF NOT EXISTS alert_rule_defi_config (
  id               BIGINT AUTO_INCREMENT PRIMARY KEY,
  protocol         VARCHAR(64) NOT NULL,
  version          VARCHAR(32) NOT NULL,
  chain_id         VARCHAR(32) NOT NULL,
  params           JSON,
  field            VARCHAR(64) NOT NULL,
  threshold        DOUBLE NOT NULL,
  direction        VARCHAR(8) NOT NULL,
  enabled          BOOLEAN NOT NULL DEFAULT true,
  frequency        JSON,
  recipient_email  VARCHAR(255) NOT NULL
);
