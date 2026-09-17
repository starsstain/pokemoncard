import React, { useEffect, useRef, useState } from "react";
import { animationConfig } from "../../config/animationConfig";

export function Projectile({ sprite, start, target, arcHeight, speedMultiplier, onComplete }) {
  if (!sprite) return null;

  const [position, setPosition] = useState(start);
  const completedRef = useRef(false);
  const speedRef = useRef(speedMultiplier);

  useEffect(() => {
    speedRef.current = speedMultiplier;
  }, [speedMultiplier]);

  useEffect(() => {
    let frameId;
    let previousTime;
    let progress = 0;
    completedRef.current = false;

    const animate = (time) => {
      previousTime ??= time;
      progress = Math.min(
        progress + ((time - previousTime) * speedRef.current) / animationConfig.projectileDuration,
        1,
      );
      previousTime = time;

      const x = start.x + (target.x - start.x) * progress;
      const baselineY = start.y + (target.y - start.y) * progress;
      const y = baselineY - 4 * arcHeight * progress * (1 - progress);
      setPosition({ x, y });

      if (progress < 1) {
        frameId = requestAnimationFrame(animate);
      } else if (!completedRef.current) {
        completedRef.current = true;
        onComplete();
      }
    };

    frameId = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(frameId);
  }, [arcHeight, onComplete, start, target]);

  return (
    <div
      className="projectile"
      style={{ left: toX(position.x), top: toY(position.y) }}
      aria-hidden="true"
    >
      <img className="projectile__sprite" src={sprite} alt="" draggable="false" />
    </div>
  );
}

const toX = (value) => `${(value / 320) * 100}%`;
const toY = (value) => `${(value / 240) * 100}%`;