import React from "react";
export function Pokemon({ sprite, side, isHit = false, isAttacking = false, isFainted = false, layout, alt, style }) {
  const classes = [
    "scene-object",
    "pokemon-sprite",
    `pokemon-sprite--${side}`,
    isHit ? "pokemon-sprite--hit" : "",
    isAttacking ? "pokemon-sprite--attacking" : "",
    isFainted ? "pokemon-sprite--fainted" : "",
  ].filter(Boolean).join(" ");

  return (
    <img
      className={classes}
      src={sprite}
      alt={alt}
      draggable="false"
      style={{ ...toSceneStyle(layout), ...style }}
    />
  );
}

function toSceneStyle({ x, y, width, height }) {
  return {
    left: `${(x / 320) * 100}%`, top: `${(y / 240) * 100}%`,
    width: `${(width / 320) * 100}%`, height: `${(height / 240) * 100}%`,
  };
}
