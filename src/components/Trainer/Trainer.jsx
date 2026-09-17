import React from "react";
import { battleAssets } from "../../utils/assetLoader";

export function Trainer({ frame, layout }) {
  const sprite = battleAssets.trainerFrames[frame - 1] ?? battleAssets.trainerFrames[0];
  return (
    <img
      className="scene-object trainer-sprite"
      src={sprite}
      alt="Player trainer"
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
