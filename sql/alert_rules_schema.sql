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
  recipient_email  VARCHAR(255) DEFAULT NULL,
  telegram_chat_id VARCHAR(64) DEFAULT NULL
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
  recipient_email  VARCHAR(255) DEFAULT NULL,
  telegram_chat_id VARCHAR(64) DEFAULT NULL
);

-- Prediction market alert rules (e.g., Polymarket)
-- params JSON fields: negRisk, question_id, question,
--                     condition_id, outcome (YES/NO), token_id
-- field: MIDPOINT  (threshold is compared against the CLOB midpoint price)
CREATE TABLE IF NOT EXISTS alert_rule_predict_market_config (
  id               BIGINT AUTO_INCREMENT PRIMARY KEY,
  predict_market   VARCHAR(64) NOT NULL,
  params           JSON,
  field            VARCHAR(64) NOT NULL,
  threshold        DOUBLE NOT NULL,
  direction        VARCHAR(8) NOT NULL,
  enabled          BOOLEAN NOT NULL DEFAULT true,
  frequency        JSON,
  recipient_email  VARCHAR(255) DEFAULT NULL,
  telegram_chat_id VARCHAR(64) DEFAULT NULL
);
