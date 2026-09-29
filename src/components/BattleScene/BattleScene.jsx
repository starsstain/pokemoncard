import React, { useEffect, useRef } from "react";
import { DEBUG } from "../../config/debug";
import { battleLayout } from "../../config/battleLayout";
import { pokemonStats } from "../../config/pokemonStats";
import { useThrowAnimation } from "../../hooks/useThrowAnimation";
import { useBattle } from "../../hooks/useBattle";
import { battleAssets } from "../../utils/assetLoader";
import { requestDebugStatus } from "../../services/attackValidation";
import { GrassPlatform } from "../GrassPlatform/GrassPlatform";
import { HealthBar } from "../HealthBar/HealthBar";
import { Pokemon } from "../Pokemon/Pokemon";
import { Projectile } from "../Projectile/Projectile";
import { Trainer } from "../Trainer/Trainer";
import "./BattleScene.css";

export function BattleScene() {
  const animation = useThrowAnimation();
  const battle = useBattle();
  const [isDevPanelOpen, setIsDevPanelOpen] = React.useState(false);

  // 投掷物命中时对敌方宝可梦结算额外伤害
  const dealDamageRef = useRef(battle.dealDamage);
  dealDamageRef.current = battle.dealDamage;
  useEffect(() => {
    if (!animation.enemyIsHit) return;
    dealDamageRef.current("enemy", pokemonStats.player.throwBonus);
  }, [animation.enemyIsHit]);

  const someoneFainted = battle.playerHp <= 0 || battle.enemyHp <= 0;

  // 结算页出现后立即冻结：中断在途的投掷动画、清掉排队中的补扔
  const cancelAnimationRef = useRef(animation.cancelAnimation);
  cancelAnimationRef.current = animation.cancelAnimation;
  useEffect(() => {
    if (someoneFainted) cancelAnimationRef.current();
  }, [someoneFainted]);

  const handlePointerDown = (event) => {
    if (event.button !== 0) return;
    if (someoneFainted) return; // 分出胜负后禁止投掷，点"再来一局"后恢复
    animation.registerTap();
  };

  const handleDebugStatusClick = (statusCode) => {
    // The Network panel is the only result surface for these standalone requests.
    void requestDebugStatus(statusCode).catch(() => {});
  };

  return (
    <section className="battle-frame" aria-label="Pixel-art battle demo">
      <div
        className={`battle-scene ${animation.isAnimating ? "battle-scene--animating" : ""}`}
        onPointerDown={handlePointerDown}
        onContextMenu={(event) => { event.preventDefault(); battle.resetBattle(); }}
        role="application"
        aria-label={animation.isAnimating ? "Attack animation in progress" : "Click to throw an item"}
      >
        <img className="battle-background" src={battleAssets.backTallGrass} alt="Tall grass battlefield" draggable="false" />

        <div className="dev-tools" onPointerDown={(event) => event.stopPropagation()}>
          <button
            className="dev-tools__toggle"
            type="button"
            onClick={() => setIsDevPanelOpen((isOpen) => !isOpen)}
          >
            dev
          </button>

          {isDevPanelOpen && (
            <div className="dev-tools__panel">
              <span className="dev-tools__label">返回状态码：</span>
              {[200, 400, 422, 500, 502, 503, 504].map((statusCode) => (
                <button
                  key={statusCode}
                  className="dev-tools__status"
                  type="button"
                  onClick={() => handleDebugStatusClick(statusCode)}
                >
                  {statusCode}
                </button>
              ))}
            </div>
          )}
        </div>


        <HealthBar
          name={pokemonStats.enemy.name}
          hp={battle.enemyHp}
          maxHp={pokemonStats.enemy.maxHp}
          attack={pokemonStats.enemy.attack}
          side="enemy"
        />
        <HealthBar
          name={pokemonStats.player.name}
          hp={battle.playerHp}
          maxHp={pokemonStats.player.maxHp}
          attack={pokemonStats.player.attack}
          side="player"
        />

        <GrassPlatform side="enemy" layout={battleLayout.enemyPlatform} />
        <Pokemon
          sprite={battleAssets.bulbasaurFront}
          side="enemy"
          isHit={animation.enemyIsHit || battle.enemyIsHit}
          isAttacking={battle.enemyAttacking}
          hitEffect={battleAssets.attacked}
          isFainted={battle.enemyHp <= 0}
          layout={battleLayout.enemyPokemon}
          alt="Bulbasaur"
        />

        <GrassPlatform side="player" layout={battleLayout.playerPlatform} />
        <Trainer frame={animation.trainerFrame} layout={battleLayout.playerTrainer} />
        <Pokemon
          sprite={battleAssets.eeveeBack}
          side="player"
          isHit={battle.playerIsHit}
          isAttacking={battle.playerAttacking}
          hitEffect={battleAssets.attacked}
          isFainted={battle.playerHp <= 0}
          layout={battleLayout.playerPokemon}
          alt="Eevee"
        />

        <Projectile
          sprite={animation.projectileIcon}
          start={battleLayout.projectile.start}
          target={battleLayout.projectile.target}
          arcHeight={animation.projectileArcHeight}
          speedMultiplier={animation.throwSpeed}
          onComplete={animation.completeProjectile}
        />

        {someoneFainted && (
          <div className="battle-result">
            <p className={`battle-result__text ${battle.enemyHp <= 0 ? "battle-result__text--win" : "battle-result__text--lose"}`}>
              {battle.enemyHp <= 0 ? "成功！" : "失败…"}
            </p>
            <button className="battle-restart" onClick={battle.resetBattle}>
              再来一局
            </button>
          </div>
        )}

        {DEBUG && <DebugOverlay />}
      </div>
    </section>
  );
}

function DebugOverlay() {
  const { projectile, enemyPokemon } = battleLayout;
  return (
    <div className="debug-overlay" aria-hidden="true">
      <DebugDot point={projectile.start} label="start" />
      <DebugDot point={projectile.target} label="target" />
      <div className="debug-box" style={{ left: `${enemyPokemon.x / 3.2}%`, top: `${enemyPokemon.y / 2.4}%`, width: `${enemyPokemon.width / 3.2}%`, height: `${enemyPokemon.height / 2.4}%` }} />
    </div>
  );
}

function DebugDot({ point, label }) {
  return <span className="debug-dot" data-label={label} style={{ left: `${point.x / 3.2}%`, top: `${point.y / 2.4}%` }} />;
}