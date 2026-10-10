import { afterEach, describe, expect, it, vi } from "vitest";
import { requestDebugStatus, validateAttack } from "./attackValidation";

// 服务层：只测前端对契约的忠实程度——URL、方法、请求体与状态码的读法

describe("validateAttack", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("返回响应的 HTTP 状态码", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 422 })));
    expect(await validateAttack("throw", 999)).toBe(422);
  });

  it("网络失败时返回 null（无状态码）", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => {
      throw new TypeError("Failed to fetch");
    }));
    expect(await validateAttack("throw", 20)).toBeNull();
  });

  it("按契约向 battle 的校验接口 POST 两个字段", async () => {
    const fetchMock = vi.fn(async () => ({ status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await validateAttack("enemy_collision", 7);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe("http://127.0.0.1:8080/api/v1/attack-validations");
    expect(options.method).toBe("POST");
    expect(options.headers).toEqual({ "Content-Type": "application/json" });
    expect(JSON.parse(options.body)).toEqual({
      attack_type: "enemy_collision",
      expected_damage: 7,
    });
  });
});

describe("requestDebugStatus", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("向 /api/v1/dev/{status} 发不带请求体的 POST", async () => {
    const fetchMock = vi.fn(async () => ({ status: 500 }));
    vi.stubGlobal("fetch", fetchMock);

    await requestDebugStatus(500);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe("http://127.0.0.1:8080/api/v1/dev/500");
    expect(options.method).toBe("POST");
    expect(options.body).toBeUndefined();
  });
});
