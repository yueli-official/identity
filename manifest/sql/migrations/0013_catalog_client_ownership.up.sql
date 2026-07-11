ALTER TABLE oidc_clients
    ADD COLUMN managed_by TEXT NOT NULL DEFAULT '';

-- Backfill only the site clients shipped by the platform Catalog. Custom
-- authorization clients remain operator-owned and reconcile must not adopt them.
UPDATE oidc_clients
SET managed_by = 'catalog'
WHERE id IN (
    'shop-ae-web', 'resource-ae-web', 'docs-ae-web', 'blog-ai-web', 'blog-ui-web',
    'shop-ae-staging-web', 'resource-ae-staging-web', 'docs-ae-staging-web',
    'blog-ai-staging-web', 'blog-ui-staging-web'
);
