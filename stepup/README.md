# Step-up 证明消费端

`stepup` 验证由 Identity 签发、绑定具体操作的认证证明。它不授予角色，也不直接授权业务操作：消费者必须先应用
自己的 RBAC/领域策略，再针对精确 action 和 resource 验证证明。

## 消费合同

1. 使用消费者 audience、稳定 action 名、规范 resource 标识和 assurance 要求向 Identity 发起 step-up。
2. 用户完成 challenge 后，把返回的证明和原始操作一起发送给消费者。
3. 使用精确的预期 action/resource 调用 `Verifier.VerifyAndConsume`。验证覆盖 issuer、audience、RS256 签名、
   `typ=step-up+jwt`、两分钟有效期、非 recovery 认证和 SHA-256 resource 绑定。
4. 只有消费者自有 replay store 原子消费证明 JTI 后，才允许执行变更。

生产消费者应使用 `PostgreSQLReplayStore` 并拥有下列表：

```sql
CREATE TABLE step_up_proof_uses (
    jti UUID PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

过期记录可以异步清理。禁止使用“先读后写”的 replay 检查；主键插入才是原子判定。
