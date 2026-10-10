import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";

vi.mock("../services/attackValidation", () => ({
  validateAttack: vi.fn(async () => 200),
}));

import { validateAttack } from "../services/attackValidation";
import { pokemonStats } from "../config/pokemonStats";
import { defaultDamages, useBattle } from "./useBattle";

// 战斗逻辑：校验状态反应、1/3 抽样、篡改判定与重开局守卫
// 全部通过 hook 的公开返回值驱动，isActive: false 关掉自动攻击，保证测试确定性。

function renderBattle(overrides = {}) {
  return renderHook(() => useBattle({ isActive: false, ...overrides }));
}

beforeEach(() => {
  vi.mocked(validateAttack).mockReset();
  vi.mocked(validateAttack).mockImplementation(async () => 200);
});

describe("后端契约数值", () => {
  it("三种攻击的默认伤害与文档约定一致（7 / 18 / 20）", () => {
    expect(defaultDamages).toEqual({
      enemy_collision: 7,
      collision: 18,
      throw: 20,
    });
  });
});

describe("校验状态反应", () => {
  it("200 保持静默：不判篡改、不提示", () => {
    const onServiceDegraded = vi.fn();
    const { result } = renderBattle({ onServiceDegraded });

    act(() => result.current.simulateStatus(200));

    expect(result.current.isTampered).toBe(false);
    expect(onServiceDegraded).not.toHaveBeenCalled();
  });

  it("422 判定数据被篡改", () => {
    const onServiceDegraded = vi.fn();
    const { result } = renderBattle({ onServiceDegraded });

    act(() => result.current.simulateStatus(422));

    expect(result.current.isTampered).toBe(true);
    expect(onServiceDegraded).not.toHaveBeenCalled();
  });

  it.each([400, 500, 502, 503, 504, null])(
    "状态 %s 只提示，不判篡改",
    (status) => {
      const onServiceDegraded = vi.fn();
      const { result } = renderBattle({ onServiceDegraded });

      act(() => result.current.simulateStatus(status));

      expect(result.current.isTampered).toBe(false);
      expect(onServiceDegraded).toHaveBeenCalledTimes(1);
      expect(onServiceDegraded).toHaveBeenCalledWith(status);
    },
  );
});

describe("抽样上报", () => {
  it("同一种攻击累计 3 次才上报一次", () => {
    const { result } = renderBattle();

    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw"));
    expect(validateAttack).not.toHaveBeenCalled();

    act(() => result.current.submitDamage("enemy", "throw"));
    expect(validateAttack).toHaveBeenCalledTimes(1);
    expect(validateAttack).toHaveBeenCalledWith("throw", 20);
  });

  it("三种攻击各自计数，互不共享", () => {
    const { result } = renderBattle();

    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("player", "enemy_collision"));

    expect(validateAttack).not.toHaveBeenCalled();
  });
});

describe("伤害结算", () => {
  it("血量截断为 0，不出现负数", () => {
    const { result } = renderBattle();

    act(() => result.current.setDamage("throw", 999));
    act(() => result.current.submitDamage("enemy", "throw"));

    expect(result.current.enemyHp).toBe(0);
  });

  it("dev 覆盖攻击力后，上报与扣血使用同一个值", () => {
    const { result } = renderBattle();

    act(() => result.current.setDamage("throw", 999));
    expect(result.current.damages.throw).toBe(999);
    expect(result.current.damages.collision).toBe(18);

    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw"));
    expect(validateAttack).toHaveBeenLastCalledWith("throw", 999);
  });

  it("还原后回到约定伤害", () => {
    const { result } = renderBattle();

    act(() => result.current.setDamage("throw", 55));
    act(() => result.current.resetDamages());

    expect(result.current.damages).toEqual(defaultDamages);
  });
});

describe("篡改链路", () => {
  it("覆盖成错误数值后，第 3 次上报拿到 422 即判篡改", async () => {
    vi.mocked(validateAttack).mockImplementation(async () => 422);
    const { result } = renderBattle();

    act(() => result.current.setDamage("throw", 21));
    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw"));
    await act(async () => {
      result.current.submitDamage("enemy", "throw");
    });

    expect(validateAttack).toHaveBeenCalledWith("throw", 21);
    expect(result.current.isTampered).toBe(true);
  });

  it("重开后清除篡改状态", () => {
    const { result } = renderBattle();

    act(() => result.current.simulateStatus(422));
    act(() => result.current.resetBattle());

    expect(result.current.isTampered).toBe(false);
    expect(result.current.enemyHp).toBe(pokemonStats.enemy.maxHp);
  });

  it("等待判决期间重开游戏，迟到的判决不再生效", async () => {
    let resolveStatus;
    vi.mocked(validateAttack).mockImplementation(
      () => new Promise((resolve) => {
        resolveStatus = resolve;
      }),
    );
    const { result } = renderBattle();

    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw"));
    act(() => result.current.submitDamage("enemy", "throw")); // 第 3 次：上报挂起
    act(() => result.current.resetBattle()); // 判决未到就重开

    await act(async () => {
      resolveStatus(422);
    });

    expect(result.current.isTampered).toBe(false);
  });

  it("dev 演练：敌方先被清空，稍后收到判决", async () => {
    const { result } = renderBattle();

    act(() => result.current.stageTamperedAttack(Promise.resolve(422)));
    expect(result.current.enemyHp).toBe(0);

    await waitFor(() => expect(result.current.isTampered).toBe(true), {
      timeout: 2000,
    });
  });
});
