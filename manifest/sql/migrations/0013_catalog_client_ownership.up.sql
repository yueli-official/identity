ALTER TABLE oidc_clients
    ADD COLUMN managed_by TEXT NOT NULL DEFAULT '';

-- Backfill only the site clients shipped by the platform Catalog. Custom
-- authorization clients remain operator-owned and reconcile must not adopt them.
UPDATE oidc_clients
SET managed_by = 'catalog'
WHERE id IN (
    'shop-main-web', 'resource-main-web', 'docs-main-web', 'blog-ai-web', 'blog-ui-web',
    'shop-main-staging-web', 'resource-main-staging-web', 'docs-main-staging-web',
    'blog-ai-staging-web', 'blog-ui-staging-web'
);
