const battleValidationUrl = "http://127.0.0.1:8080/api/v1/attack-validations";

const debugRequests = {
  200: { body: { attack_type: "throw", expected_damage: 20 } },
  400: { body: { attack_type: "invalid", expected_damage: 20 } },
  422: { body: { attack_type: "throw", expected_damage: 999 } },
  500: { body: { attack_type: "throw", expected_damage: 20 }, fault: "attack-500" },
  502: { body: { attack_type: "throw", expected_damage: 20 }, fault: "battle-502" },
  503: { body: { attack_type: "throw", expected_damage: 20 }, fault: "battle-503" },
  504: { body: { attack_type: "throw", expected_damage: 20 }, fault: "attack-timeout" },
};

// Sends a standalone validation request for inspection in the browser Network panel.
// The response body is intentionally not read because the API contract is status-only.
export function requestDebugStatus(statusCode) {
  const request = debugRequests[statusCode];
  if (!request) return Promise.reject(new Error(`Unsupported debug status: ${statusCode}`));

  const headers = { "Content-Type": "application/json" };
  if (request.fault) headers["X-Pokemon-Debug-Fault"] = request.fault;

  return fetch(battleValidationUrl, {
    method: "POST",
    headers,
    body: JSON.stringify(request.body),
  });
}
