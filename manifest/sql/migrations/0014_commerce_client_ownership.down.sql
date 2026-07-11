UPDATE oidc_clients
SET managed_by = ''
WHERE id = 'commerce-svc'
  AND managed_by = 'catalog';
