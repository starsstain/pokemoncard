import { useCallback, useEffect, useRef, useState } from "react";
import { battleConfig, pokemonStats } from "../config/pokemonStats";
import { validateAttack } from "../services/attackValidation";

// 各攻击类型的名义伤害默认值，须与后端约定一致（enemy_collision=7 / collision=18 / throw=20）；
// dev 面板可在运行时覆盖，用于演练篡改检测。
export const defaultDamages = {
  enemy_collision: pokemonStats.player.attack,
  collision: pokemonStats.enemy.attack,
  throw: pokemonStats.player.throwBonus,
};

// 攻击方到 attack_type 的映射
const attackTypes = {
  player: "enemy_collision",
  enemy: "collision",
};

// 抽样上报比例：三种攻击各自计数，每 3 次传 1 次，降低服务端压力
const reportEvery = 3;

// dev 演练参数：被篡改的一击先真实生效，隔一段“校验耗时”才收到判决，还原真实链路的先后节奏
const tamperRehearsedDamage = 999;
const tamperVerdictDelay = 900;

// 战斗状态：血量由前端独立结算，不等后端；
// 校验请求按抽样异步发出：400/422 判定“数据被篡改”（必须重开），5xx/网络故障仅提示。
export function useBattle({ onEnemyHit, onServiceDegraded, isActive = true } = {}) {
  const [battle, setBattle] = useState({
    playerHp: pokemonStats.player.maxHp,
    enemyHp: pokemonStats.enemy.maxHp,
    playerAttacking: false,
    enemyAttacking: false,
    enemyIsHit: false,
    playerIsHit: false,
    isTampered: false,
  });

  const battleRef = useRef(battle);
  const mountedRef = useRef(true);
  const timeoutRef = useRef(null);
  const roundRef = useRef(0); // 局数：重开后递增，使在途校验结果作废
  const reportCountersRef = useRef({}); // 各攻击类型的累计次数，各自每 3 次上报 1 次
  const [damages, setDamagesState] = useState({ ...defaultDamages }); // 当前生效的名义伤害（dev 可覆盖，血条 ATK 同步显示）
  const damageRef = useRef(damages);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      window.clearTimeout(timeoutRef.current);
    };
  }, []);

  const updateBattle = useCallback((patch) => {
    battleRef.current = { ...battleRef.current, ...patch };
    setBattle(battleRef.current);
  }, []);

  const wait = useCallback((duration) => new Promise((resolve) => {
    timeoutRef.current = window.setTimeout(resolve, duration);
  }), []);

  // 校验状态的统一反应：200 静默；422 说明上报数值对不上约定，判定篡改；
  // 400 是请求格式问题（多为前端 bug），5xx 与网络失败（status 为 null）是服务或通信问题，均仅提示。
  const reactToStatus = useCallback((status) => {
    if (status === 200) return;
    if (status === 422) {
      updateBattle({ isTampered: true }); // 本局数据不可信，必须重开
      return;
    }
    onServiceDegraded?.(status);
  }, [onServiceDegraded, updateBattle]);

  // 真实校验：抽样发出的请求回来后走统一反应
  const checkReport = useCallback(async (attackType, damage) => {
    const round = roundRef.current;
    const status = await validateAttack(attackType, damage);
    if (!mountedRef.current || round !== roundRef.current) return; // 期间重开游戏，旧结果不影响新一局
    reactToStatus(status);
  }, [reactToStatus]);

  // dev 面板模拟一条校验结果，与真实校验走同一套反应
  const simulateStatus = useCallback((status) => {
    reactToStatus(status);
  }, [reactToStatus]);

  // dev 面板：覆盖/还原某类攻击的名义伤害（扣血、上报与血条 ATK 显示同步使用该值）
  const setDamage = useCallback((attackType, value) => {
    const next = { ...damageRef.current, [attackType]: value };
    damageRef.current = next;
    setDamagesState(next);
  }, []);

  const resetDamages = useCallback(() => {
    damageRef.current = { ...defaultDamages };
    setDamagesState(damageRef.current);
  }, []);

  // 抽样上报：三种攻击各自计数，每累计满 3 次就上报“当前这次”
  const reportSampled = useCallback((attackType, damage) => {
    const count = (reportCountersRef.current[attackType] ?? 0) + 1;
    reportCountersRef.current[attackType] = count;
    if (count % reportEvery !== 0) return;
    void checkReport(attackType, damage);
  }, [checkReport]);

  // 通用攻击流程：冲向对手 -> 碰撞瞬间独立扣血并抽样上报 -> 弹回
  const runAttack = useCallback(async (attacker, defender) => {
    const attackerKey = `${attacker}Attacking`;
    const defenderKey = `${defender}IsHit`;
    const round = roundRef.current;

    updateBattle({ [attackerKey]: true });
    await wait(battleConfig.attackDashDuration);
    if (!mountedRef.current || round !== roundRef.current || battleRef.current.isTampered) return;

    const attackType = attackTypes[attacker];
    const damage = damageRef.current[attackType];
    updateBattle({
      [defenderKey]: true,
      [`${defender}Hp`]: Math.max(0, battleRef.current[`${defender}Hp`] - damage),
    });

    if (defender === "enemy") onEnemyHit?.();
    reportSampled(attackType, damage);

    // 碰撞后停顿一下再弹回，让撞击感更明显
    await wait(battleConfig.attackReturnDuration);
    if (!mountedRef.current) return;
    updateBattle({ [attackerKey]: false });

    await wait(battleConfig.hitFlashDuration - battleConfig.attackReturnDuration);
    if (!mountedRef.current) return;
    updateBattle({ [defenderKey]: false });
  }, [onEnemyHit, reportSampled, updateBattle, wait]);

  // 双方自动攻击循环：任一方倒下或数据被判篡改后停止
  useEffect(() => {
    if (!isActive) return undefined;

    let playerTimer = window.setTimeout(function schedulePlayerAttack() {
      const isBattleActive = battleRef.current.playerHp > 0 && battleRef.current.enemyHp > 0 && !battleRef.current.isTampered;
      const isAttackInProgress = battleRef.current.playerAttacking || battleRef.current.enemyAttacking;

      if (isBattleActive && !isAttackInProgress) {
        runAttack("player", "enemy");
      }

      const nextDelay = isBattleActive && isAttackInProgress
        ? battleConfig.attackRetryDelay
        : battleConfig.autoAttackInterval;
      playerTimer = window.setTimeout(schedulePlayerAttack, nextDelay);
    }, battleConfig.autoAttackInterval);

    let enemyTimer = window.setTimeout(function scheduleEnemyAttack() {
      const isBattleActive = battleRef.current.playerHp > 0 && battleRef.current.enemyHp > 0 && !battleRef.current.isTampered;
      const isAttackInProgress = battleRef.current.playerAttacking || battleRef.current.enemyAttacking;

      if (isBattleActive && !isAttackInProgress) {
        runAttack("enemy", "player");
      }

      const nextDelay = isBattleActive && isAttackInProgress
        ? battleConfig.attackRetryDelay
        : battleConfig.autoAttackInterval;
      enemyTimer = window.setTimeout(scheduleEnemyAttack, nextDelay);
    }, battleConfig.autoAttackInterval / 2);

    return () => {
      window.clearTimeout(playerTimer);
      window.clearTimeout(enemyTimer);
    };
  }, [isActive, runAttack]);

  // 扣血与受击反馈：血量截断为 0，闪白由调用方收尾
  const dealDamage = useCallback((defender, amount) => {
    const hp = Math.max(0, battleRef.current[`${defender}Hp`] - amount);
    updateBattle({ [`${defender}Hp`]: hp, [`${defender}IsHit`]: true });
    timeoutRef.current = window.setTimeout(() => {
      if (mountedRef.current) updateBattle({ [`${defender}IsHit`]: false });
    }, battleConfig.hitFlashDuration);
    return hp;
  }, [updateBattle]);

  // dev 面板演练完整作弊链路：被篡改的一击先真实生效（清空血量、先看到“成功”），
  // 隔一段校验耗时再收到判决，避免弹窗凭空出现。
  const stageTamperedAttack = useCallback((statusPromise) => {
    if (battleRef.current.isTampered) return;
    const round = roundRef.current;
    dealDamage("enemy", tamperRehearsedDamage);
    const verdictDelay = new Promise((resolve) => window.setTimeout(resolve, tamperVerdictDelay));
    void Promise.all([Promise.resolve(statusPromise).catch(() => null), verdictDelay])
      .then(([status]) => {
        if (!mountedRef.current || round !== roundRef.current) return; // 期间重开，判决不再生效
        reactToStatus(status);
      });
  }, [dealDamage, reactToStatus]);

  // 手动伤害入口（投掷物命中时调用）：立即扣血，再按抽样决定是否上报
  const submitDamage = useCallback((defender, attackType) => {
    if (battleRef.current.isTampered) return;
    const damage = damageRef.current[attackType];
    dealDamage(defender, damage);
    reportSampled(attackType, damage);
  }, [dealDamage, reportSampled]);

  // 倒下或篡改后重置战斗；局数递增，使在途请求的结果不再生效
  const resetBattle = useCallback(() => {
    roundRef.current += 1;
    updateBattle({
      playerHp: pokemonStats.player.maxHp,
      enemyHp: pokemonStats.enemy.maxHp,
      playerAttacking: false,
      enemyAttacking: false,
      enemyIsHit: false,
      playerIsHit: false,
      isTampered: false,
    });
  }, [updateBattle]);

  return { ...battle, damages, submitDamage, resetBattle, simulateStatus, stageTamperedAttack, setDamage, resetDamages, wait };
}
