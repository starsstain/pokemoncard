import React from "react";
export function Pokemon({ sprite, side, isHit = false, layout, alt }) {
  return (
    <img
      className={`scene-object pokemon-sprite pokemon-sprite--${side} ${isHit ? "pokemon-sprite--hit" : ""}`}
      src={sprite}
      alt={alt}
      draggable="false"
      style={toSceneStyle(layout)}
    />
  );
}

function toSceneStyle({ x, y, width, height }) {
  return {
    left: `${(x / 320) * 100}%`, top: `${(y / 240) * 100}%`,
    width: `${(width / 320) * 100}%`, height: `${(height / 240) * 100}%`,
  };
}
