import React from "react";
import { battleAssets } from "../../utils/assetLoader";

export function GrassPlatform({ side, layout }) {
  return (
    <img
      className={`scene-object grass-platform grass-platform--${side}`}
      src={battleAssets.groundTallGrass}
      alt=""
      draggable="false"
      style={toSceneStyle(layout)}
    />
  );
}

function toSceneStyle({ x, y, width, height }) {
  return {
    left: `${(x / 320) * 100}%`,
    top: `${(y / 240) * 100}%`,
    width: `${(width / 320) * 100}%`,
    height: `${(height / 240) * 100}%`,
  };
}
