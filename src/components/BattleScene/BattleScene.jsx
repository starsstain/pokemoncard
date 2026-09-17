import React from "react";
import { DEBUG } from "../../config/debug";
import { battleLayout } from "../../config/battleLayout";
import { useThrowAnimation } from "../../hooks/useThrowAnimation";
import { battleAssets } from "../../utils/assetLoader";
import { GrassPlatform } from "../GrassPlatform/GrassPlatform";
import { Pokemon } from "../Pokemon/Pokemon";
import { Projectile } from "../Projectile/Projectile";
import { Trainer } from "../Trainer/Trainer";
import "./BattleScene.css";

export function BattleScene() {
  const animation = useThrowAnimation();

  const handlePointerDown = (event) => {
    if (event.button !== 0) return;
    animation.registerTap();
  };

  return (
    <section className="battle-frame" aria-label="Pixel-art battle demo">
      <div
        className={`battle-scene ${animation.isAnimating ? "battle-scene--animating" : ""}`}
        onPointerDown={handlePointerDown}
        role="application"
        aria-label={animation.isAnimating ? "Attack animation in progress" : "Click to throw an item"}
      >
        <img className="battle-background" src={battleAssets.backTallGrass} alt="Tall grass battlefield" draggable="false" />

        <GrassPlatform side="enemy" layout={battleLayout.enemyPlatform} />
        <Pokemon sprite={battleAssets.bulbasaurFront} side="enemy" isHit={animation.enemyIsHit} layout={battleLayout.enemyPokemon} alt="Bulbasaur" />

        <GrassPlatform side="player" layout={battleLayout.playerPlatform} />
        <Trainer frame={animation.trainerFrame} layout={battleLayout.playerTrainer} />
        <Pokemon sprite={battleAssets.eeveeBack} side="player" layout={battleLayout.playerPokemon} alt="Eevee" />

        <Projectile
          sprite={animation.projectileIcon}
          start={battleLayout.projectile.start}
          target={battleLayout.projectile.target}
          arcHeight={animation.projectileArcHeight}
          speedMultiplier={animation.throwSpeed}
          onComplete={animation.completeProjectile}
        />

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