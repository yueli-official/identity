UPDATE oidc_clients
   SET scopes      = '{openid,profile,email,roles}',
       grant_types = '{authorization_code}'
 WHERE id = 'demo-web';
DROP TABLE IF EXISTS oidc_refresh_tokens;
DROP TABLE IF EXISTS oidc_oauth_requests;
