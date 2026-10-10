const battleValidationUrl = "http://127.0.0.1:8080/api/v1/attack-validations";
const battleDevStatusUrl = "http://127.0.0.1:8080/api/v1/dev";

// 发起一次攻击校验：契约只有 HTTP 状态码，响应体为空、不解析。
// 网络失败（battle 未启动或连接中断）时返回 null，由调用方按“无状态码”处理、不扣血。
export async function validateAttack(attackType, expectedDamage) {
  try {
    const response = await fetch(battleValidationUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ attack_type: attackType, expected_damage: expectedDamage }),
    });
    return response.status;
  } catch {
    return null;
  }
}

// Sends a standalone dev-panel request. The contract is status-only with an empty body:
// no request body, no response body to read — the caller reacts to response.status alone.
export function requestDebugStatus(statusCode) {
  return fetch(`${battleDevStatusUrl}/${statusCode}`, { method: "POST" });
}
