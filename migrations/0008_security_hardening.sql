ALTER TABLE auth_flows
    ADD COLUMN browser_binding_hash TEXT NOT NULL DEFAULT '';

INSERT INTO schema_migrations(version, name)
VALUES (8, 'security_hardening');
