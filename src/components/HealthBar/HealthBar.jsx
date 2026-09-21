import React from "react";
import { getHealthColor } from "../../config/pokemonStats";
import "./HealthBar.css";

export function HealthBar({ name, hp, maxHp, attack, side }) {
  const ratio = Math.max(0, Math.min(1, hp / maxHp));
  const low = ratio <= 0.2;

  return (
    <div className={`health-bar health-bar--${side} ${low ? "health-bar--low" : ""}`}>
      <span className="health-bar__name">{name}</span>
      <div className="health-bar__track">
        <div
          className="health-bar__fill"
          style={{ width: `${ratio * 100}%`, backgroundColor: getHealthColor(ratio) }}
        />
      </div>
      <span className="health-bar__stats">
        HP {Math.max(0, Math.round(hp))}/{maxHp} · ATK {attack}
      </span>
    </div>
  );
}
