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