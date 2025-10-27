INSERT INTO accounts (account_id, tenant_id, email_hash, email_cipher, disabled, created_at, updated_at)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    '22222222-2222-2222-2222-222222222222',
    DECODE('7462108984f629db2ced1aeb2dc3e747e53a2e1c607059f72955ab864c724335', 'hex'),
    'demo@example.com',
    FALSE,
    NOW(),
    NOW()
)
ON CONFLICT (account_id) DO NOTHING;
