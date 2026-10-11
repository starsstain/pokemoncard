import React, { useEffect, useState } from "react";

export function Pokemon({ sprite, side, isHit = false, isAttacking = false, isFainted = false, layout, alt, style, hitEffect }) {
  const [hitEffectPosition, setHitEffectPosition] = useState(null);

  useEffect(() => {
    if (!isHit) {
      setHitEffectPosition(null);
      return;
    }

    setHitEffectPosition({
      x: 28 + Math.random() * 44,
      y: 30 + Math.random() * 40,
    });
  }, [isHit]);
  const classes = [
    "scene-object",
    "pokemon-sprite",
    `pokemon-sprite--${side}`,
    isHit ? "pokemon-sprite--hit" : "",
    isAttacking ? "pokemon-sprite--attacking" : "",
    isFainted ? "pokemon-sprite--fainted" : "",
  ].filter(Boolean).join(" ");

  return (
    <>
      <img
        className={classes}
        src={sprite}
        alt={alt}
        draggable="false"
        style={{ ...toSceneStyle(layout), ...style }}
      />
      {isHit && hitEffect && hitEffectPosition && (
        <img
          className="pokemon-hit-effect"
          src={hitEffect}
          alt=""
          aria-hidden="true"
          draggable="false"
          style={toHitEffectStyle(layout, hitEffectPosition)}
        />
      )}
    </>
  );
}

function toSceneStyle({ x, y, width, height }) {
  return {
    left: `${(x / 320) * 100}%`, top: `${(y / 240) * 100}%`,
    width: `${(width / 320) * 100}%`, height: `${(height / 240) * 100}%`,
  };
}

function toHitEffectStyle({ x, y, width, height }, position) {
  return {
    left: `${((x + width * position.x / 100) / 320) * 100}%`,
    top: `${((y + height * position.y / 100) / 240) * 100}%`,
    width: `${((width * 0.75) / 320) * 100}%`,
  };
}
