import { useCallback, useEffect, useRef, useState } from "react";
import { battleConfig, pokemonStats } from "../config/pokemonStats";
import { getRandomInteger } from "../utils/random";

// 战斗状态：双方生命值、自动攻击与受击反馈。
export function useBattle({ onEnemyHit, isActive = true } = {}) {
  const [battle, setBattle] = useState({
    playerHp: pokemonStats.player.maxHp,
    enemyHp: pokemonStats.enemy.maxHp,
    playerAttacking: false,
    enemyAttacking: false,
    enemyIsHit: false,
    playerIsHit: false,
  });

  const battleRef = useRef(battle);
  const mountedRef = useRef(true);
  const timeoutRef = useRef(null);

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

  // 通用攻击流程：冲向对手 -> 碰撞瞬间结算伤害 -> 弹回
  const runAttack = useCallback(async (attacker, defender) => {
    const attackerKey = `${attacker}Attacking`;
    const defenderKey = `${defender}IsHit`;

    updateBattle({ [attackerKey]: true });
    await wait(battleConfig.attackDashDuration);
    if (!mountedRef.current) return;

    const damage = getRandomInteger(
      Math.round(pokemonStats[attacker].attack * 0.75),
      Math.round(pokemonStats[attacker].attack * 1.25),
    );
    updateBattle({
      [defenderKey]: true,
      [`${defender}Hp`]: Math.max(0, battleRef.current[`${defender}Hp`] - damage),
    });

    if (defender === "enemy") onEnemyHit?.();

    // 碰撞后停顿一下再弹回，让撞击感更明显
    await wait(battleConfig.attackReturnDuration);
    if (!mountedRef.current) return;
    updateBattle({ [attackerKey]: false });

    await wait(battleConfig.hitFlashDuration - battleConfig.attackReturnDuration);
    if (!mountedRef.current) return;
    updateBattle({ [defenderKey]: false });
  }, [onEnemyHit, updateBattle, wait]);

  // 双方自动攻击循环：任一方倒下后停止
  useEffect(() => {
    if (!isActive) return undefined;

    let playerTimer = window.setTimeout(function schedulePlayerAttack() {
      const isBattleActive = battleRef.current.playerHp > 0 && battleRef.current.enemyHp > 0;
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
      const isBattleActive = battleRef.current.playerHp > 0 && battleRef.current.enemyHp > 0;
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

  // 手动伤害入口（投掷物命中时调用）
  const dealDamage = useCallback((defender, amount) => {
    const hp = Math.max(0, battleRef.current[`${defender}Hp`] - amount);
    updateBattle({ [`${defender}Hp`]: hp, [`${defender}IsHit`]: true });
    timeoutRef.current = window.setTimeout(() => {
      if (mountedRef.current) updateBattle({ [`${defender}IsHit`]: false });
    }, battleConfig.hitFlashDuration);
    return hp;
  }, [updateBattle]);

  // 倒下后重置战斗
  const resetBattle = useCallback(() => {
    updateBattle({
      playerHp: pokemonStats.player.maxHp,
      enemyHp: pokemonStats.enemy.maxHp,
      playerAttacking: false,
      enemyAttacking: false,
      enemyIsHit: false,
      playerIsHit: false,
    });
  }, [updateBattle]);

  return { ...battle, dealDamage, resetBattle, wait };
}
