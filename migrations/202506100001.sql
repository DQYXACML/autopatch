DO $$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'uint256') THEN
            CREATE DOMAIN UINT256 AS NUMERIC
                CHECK (VALUE >= 0 AND VALUE < POWER(CAST(2 AS NUMERIC), CAST(256 AS NUMERIC)) AND SCALE(VALUE) = 0);
        ELSE
            ALTER DOMAIN UINT256 DROP CONSTRAINT uint256_check;
            ALTER DOMAIN UINT256 ADD
                CHECK (VALUE >= 0 AND VALUE < POWER(CAST(2 AS NUMERIC), CAST(256 AS NUMERIC)) AND SCALE(VALUE) = 0);
        END IF;
    END $$;


CREATE TABLE IF NOT EXISTS block_headers (
                                             hash  VARCHAR PRIMARY KEY,
                                             parent_hash VARCHAR NOT NULL UNIQUE,
                                             number UINT256 NOT NULL UNIQUE,
                                             timestamp INTEGER NOT NULL UNIQUE CHECK(timestamp > 0),
                                             rlp_bytes VARCHAR NOT NULL
);
CREATE INDEX IF NOT EXISTS block_headers_number ON block_headers(number);

CREATE TABLE IF NOT EXISTS Members (
                                       guid                          VARCHAR PRIMARY KEY,
                                       member                        VARCHAR NOT NULL,
                                       is_active                     SMALLINT NOT NULL DEFAULT 0,
                                       timestamp                     INTEGER NOT NULL CHECK (timestamp > 0)
);


CREATE TABLE IF NOT EXISTS protected_created (
                                                 guid                          VARCHAR PRIMARY KEY,
                                                 protected_address                 VARCHAR NOT NULL,
                                                 timestamp                     INTEGER NOT NULL CHECK (timestamp > 0)
);
CREATE INDEX IF NOT EXISTS protected_created_protected_address ON protected_created(protected_address);

CREATE TABLE IF NOT EXISTS attack_tx (
                                         guid VARCHAR PRIMARY KEY,
                                         tx_hash VARCHAR NOT NULL,
                                         block_number UINT256 NOT NULL  CHECK(block_number>0),
                                         block_hash VARCHAR,
                                         contract_address VARCHAR,
                                         from_address VARCHAR,
                                         to_address VARCHAR,
                                         value UINT256,
                                         gas_used UINT256,
                                         gas_price UINT256,
                                         status INTEGER DEFAULT 0,
                                         attack_type VARCHAR,
                                         error_message TEXT,
                                         created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                         updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                         timestamp INTEGER NOT NULL CHECK(timestamp>0)
);
CREATE INDEX IF NOT EXISTS attack_tx_hash ON attack_tx(tx_hash);
CREATE INDEX IF NOT EXISTS attack_tx_contract ON attack_tx(contract_address);
CREATE INDEX IF NOT EXISTS attack_tx_status ON attack_tx(status);
CREATE INDEX IF NOT EXISTS attack_tx_attack_type ON attack_tx(attack_type);
CREATE INDEX IF NOT EXISTS attack_tx_created_at ON attack_tx(created_at);

CREATE TABLE IF NOT EXISTS protected_storage (
                                                 guid VARCHAR PRIMARY KEY,
                                                 protected_address                 VARCHAR NOT NULL,
                                                 storage_key VARCHAR NOT NULL,
                                                 storage_value VARCHAR NOT NULL,
                                                 number UINT256 NOT NULL
);
CREATE INDEX IF NOT EXISTS protected_storage_protected_address ON protected_storage(protected_address);
CREATE INDEX IF NOT EXISTS protected_storage_storage_key ON protected_storage(storage_key);

CREATE TABLE IF NOT EXISTS protected_invariants (
                                                    guid                          VARCHAR PRIMARY KEY,
                                                    contract_address VARCHAR NOT NULL,
                                                    invariant_expression TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS protected_invariants_contract_address ON protected_invariants(contract_address);

CREATE TABLE IF NOT EXISTS protected_txs (
                                             guid                VARCHAR PRIMARY KEY,
                                             block_hash          VARCHAR           NOT NULL,
                                             hash                VARCHAR           NOT NULL,
                                             block_number        UINT256           NOT NULL,
                                             protected_address   VARCHAR           NOT NULL,
                                             input_data        VARCHAR           NOT NULL
);
CREATE INDEX IF NOT EXISTS protected_txs_protected_address ON protected_txs (protected_address);
CREATE INDEX IF NOT EXISTS protected_txs_block_hash ON protected_txs (block_hash);
CREATE INDEX IF NOT EXISTS protected_txs_block_number ON protected_txs (block_number);
CREATE INDEX IF NOT EXISTS protected_txs_hash ON protected_txs (hash);

CREATE TABLE IF NOT EXISTS addresses (
                                         guid  VARCHAR PRIMARY KEY,
                                         user_uid  VARCHAR NOT NULL,
                                         address VARCHAR NOT NULL,
                                         address_type SMALLINT NOT NULL DEFAULT 0,
                                         private_key VARCHAR NOT NULL,
                                         public_key VARCHAR NOT NULL,
                                         timestamp INTEGER NOT NULL CHECK(timestamp>0)
);
CREATE INDEX IF NOT EXISTS addresses_user_uid ON addresses(user_uid);
CREATE INDEX IF NOT EXISTS addresses_address ON addresses(address);
CREATE INDEX IF NOT EXISTS addresses_timestamp ON addresses(timestamp);